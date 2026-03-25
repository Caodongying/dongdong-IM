package pool

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Caodongying/dongdong-IM/utils/config"
	"github.com/Caodongying/dongdong-IM/utils/logger"
	"github.com/Caodongying/dongdong-IM/utils/trace"
	"go.uber.org/zap"
)


var (
	GlobalPool *GoroutinePool

	// 错误信息是固定的用 errors.New，需要带变量用 fmt.Errorf
	ErrPoolFull    = errors.New("goroutine pool is full")
	ErrPoolStopped = errors.New("goroutine pool is stopped")
	ErrNilTask     = errors.New("task func is nil")
)

type GoroutinePool struct {
	// 核心参数
	coreWorkers int32         // 核心协程数（常驻，不会因空闲被回收）
	maxWorkers  int32         // 最大协程数（核心 + 临时）
	idleTimeout time.Duration // 临时协程空闲超时，超时后自动退出

	taskQueue chan func() // 任务队列（有缓冲 channel）

	// 【八股：atomic vs mutex】
	// 对于简单的整数计数，atomic 比 mutex 性能更好：
	//   - atomic：CPU 级别的 CAS 指令，无锁，纳秒级
	//   - mutex：需要系统调用（futex），涉及用户态/内核态切换，微秒级
	// 但 atomic 只适合独立变量的操作，多个变量需要一致性时仍需 mutex

	workerNum atomic.Int32 // 当前运行中的 worker 数量

	// 与协程池关闭有关的值
	wg sync.WaitGroup
	stopped atomic.Bool // 停止标记

	// 监控指标
	totalSubmitted atomic.Int64
	totalCompleted atomic.Int64
	totalPanic     atomic.Int64
}

func NewPool(coreWorkers, maxWorkers, queueSize int, idleTimeout time.Duration) (*GoroutinePool, error) {
	if coreWorkers <= 0 || maxWorkers < coreWorkers || queueSize <= 0 {
		return nil, fmt.Errorf("invalid pool config: core=%d, max=%d, queue=%d",
			coreWorkers, maxWorkers, queueSize)
	}

	p := &GoroutinePool{
		coreWorkers: int32(coreWorkers),
		maxWorkers:  int32(maxWorkers),
		idleTimeout: idleTimeout,
		taskQueue:   make(chan func(), queueSize),
	}

	// 启动核心 worker
	// 核心 worker 是常驻的，不会因空闲超时退出
	for i := 0; i < coreWorkers; i++ {
		p.workerNum.Add(1)
		p.wg.Add(1)
		go p.coreWorker()
	}

	return p, nil
}

// Init 全局初始化（兼容现有调用方式）
func Init() error {
	cfg := config.Viper.GetConfig().Pool
	p, err := NewPool(cfg.CoreSize, cfg.MaxSize, cfg.QueueSize,
		time.Duration(cfg.IdleTimeout)*time.Second)
	if err != nil {
		logger.Sugar.Error("init goroutine pool failed", zap.Error(err))
		return err
	}
	GlobalPool = p
	logger.Sugar.Info("goroutine pool init success",
		zap.Int("core_workers", cfg.CoreSize),
		zap.Int("max_workers", cfg.MaxSize),
		zap.Int("queue_size", cfg.QueueSize),
		zap.Int("idle_timeout_sec", cfg.IdleTimeout))
	return nil
}


func (p *GoroutinePool) coreWorker() {
	defer p.wg.Done()
	defer p.workerNum.Add(-1)

	for task := range p.taskQueue {
		p.runTask(task)
	}
}

// tempWorker 临时工作协程（空闲超时后自动退出）
//
// 当队列满且 worker 数未达上限时，会创建临时 worker 来处理突发流量。
// 临时 worker 与核心 worker 的区别：空闲时间超过 idleTimeout 就自动退出，
// 避免突发流量过后 goroutine 空占资源。
//
// 实现方式是 select 同时监听 taskQueue 和 time.After：
//   - 收到任务 → 执行任务，重置超时
//   - 超时触发 → 退出 goroutine
//
// 这就是 Go 中经典的"超时控制"模式。
func (p *GoroutinePool) tempWorker() {
	defer p.wg.Done()
	defer p.workerNum.Add(-1)

	idleTimer := time.NewTimer(p.idleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case task, ok := <-p.taskQueue:
			if !ok {
				// channel 已关闭，退出
				return
			}
			p.runTask(task)

			// 【八股：time.Timer 的正确 Reset 方式】
			// Go 官方文档指出：对已触发或已停止的 Timer 调用 Reset 前，
			// 需要先 drain channel，否则可能立即收到过期的旧信号。
			// 这里因为我们刚从 select 的另一个 case 走过来（不是 timer case），
			// timer 可能已触发也可能没触发，所以需要安全地 drain。
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(p.idleTimeout)

		case <-idleTimer.C:
			logger.Sugar.Debug("空闲超时，临时 worker 退出",
				zap.Int32("current_workers", p.workerNum.Load()))
			return
		}
	}
}

// runTask 执行单个任务（带 panic recovery）
//
// goroutine 的 panic 如果没有被 recover 捕获（这里是指运行task()时可能遇到的panic），会导致整个进程崩溃。
// 在协程池中，worker 执行的是用户提交的任务函数，我们无法保证它不会 panic。
// 因此必须在执行任务时 defer recover()，将 panic 转化为错误日志，
// 保证 worker goroutine 存活继续服务。
func (p *GoroutinePool) runTask(task func()) {
	defer func() {
		p.totalCompleted.Add(1)
		if r := recover(); r != nil {
			p.totalPanic.Add(1)
			// 【八股：debug.Stack() vs runtime.Stack()】
			// debug.Stack() 是 runtime.Stack() 的便捷封装，返回完整的堆栈跟踪。
			// 线上排查 panic 时，没有堆栈等于没有信息，必须打印。
			logger.Sugar.Error("task panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())))
		}
	}()
	task()
}

// Submit 提交任务到协程池
//
// 本实现采用三级策略：
//   1. 尝试直接入队（非阻塞 select + default）
//   2. 队列满 → 尝试创建临时 worker 并入队
//   3. 临时 worker 也满 → 返回 ErrPoolFull
func (p *GoroutinePool) Submit(ctx context.Context, taskFunc func(ctx context.Context)) error {
	// 参数校验
	if taskFunc == nil {
		return ErrNilTask
	}
	// 判断协程池是否已经关闭
	if p.stopped.Load() {
		return ErrPoolStopped
	}
	// 将带context的task函数封装为不带参数的闭包，为入队做准备
	task := func() {
		// 判断context是否到期
		select {
		case <-ctx.Done():
			logger.Sugar.Warn("task skipped: context cancelled",
				zap.String("trace_id", trace.GetTraceID(ctx)),
				zap.Error(ctx.Err()))
			return
		default:
			// 执行task
			taskFunc(ctx)
		}
	}

	p.totalSubmitted.Add(1)

	// 第一步：尝试直接入队（非阻塞）
	// 如果 channel 有空间，发送成功；否则立即走 default，不阻塞调用方。
	select {
	case p.taskQueue <- task:
		return nil
	default:
	}

	// 第二步：如果任务队列满了，但是还有空闲的临时worker，启用（扩容）
	// 【八股：CAS 自旋实现无锁扩容】
	// 用 for 循环 + CompareAndSwap 实现乐观锁：
	//   - Load 当前值 → 判断是否可扩容 → CAS 尝试 +1
	//   - 如果其他 goroutine 抢先修改了值，CAS 失败，重新 Load 再试
	//   - 比 mutex 开销小，适合竞争不激烈的场景
	for {
		current := p.workerNum.Load()
		if current >= p.maxWorkers {
			break // 已达上限，无法扩容
		}
		if p.workerNum.CompareAndSwap(current, current+1) {
			p.wg.Add(1)
			go p.tempWorker()

			// 扩容后再次尝试入队
			select {
			case p.taskQueue <- task:
				return nil
			default:
				// 极端情况：扩了 worker 但队列仍满（其他 goroutine 同时提交）
				// 不回滚 worker——让它自己空闲超时退出即可
				// 回滚 workerNum 但不停止已启动的 goroutine 会导致计数不一致
			}
			break
		}
		// CAS 失败，继续自旋重试
	}

	// 第三步：最后再尝试一次入队（刚创建的 worker 可能已消费了一个任务，腾出了空间）
	select {
	case p.taskQueue <- task:
		return nil
	default:
		return ErrPoolFull
	}
}

// Close 优雅关闭协程池
// 【八股：如何优雅关闭 goroutine？】
// Go 中没有办法从外部"杀死"一个 goroutine，只能通过信号让它自己退出。
// 常见的信号机制：
//   - close(channel)：让所有 for range 该 channel 的 goroutine 退出循环
//   - context.Cancel()：通过 ctx.Done() channel 通知
//   - 专用的 quit channel
//
// 本实现的关闭流程：
//   1. 设置 stopped 标记，拒绝新任务提交
//   2. close(taskQueue)，worker 的 for range 会 drain 完剩余任务后退出
//   3. WaitGroup.Wait() 等待所有 worker 退出（带超时保护）
//
// close channel 的优势：剩余任务会被 drain 完，不会丢任务。
func (p *GoroutinePool) Close(waitTime time.Duration) error {
	// 【八股：CAS 保证只关闭一次】
	// CompareAndSwap(false, true) 是线程安全的"只执行一次"模式。
	// 多个 goroutine 同时调用 Close，只有一个会成功。
	if !p.stopped.CompareAndSwap(false, true) {
		return ErrPoolStopped
	}

	logger.Sugar.Info("关闭协程池, 处理剩余task...")

	// 关闭 taskQueue channel
	// 所有 worker（core 和 temp）都在 range/select 这个 channel，
	// close 后它们会 drain 完剩余任务再退出。
	close(p.taskQueue)

	// 带超时等待所有 worker 退出
	// 【八股：WaitGroup + select 实现带超时的等待】
	// WaitGroup 本身不支持超时，但可以用一个 goroutine + channel 包装：
	//   - 启动一个 goroutine 执行 wg.Wait()，完成后往 done channel 发信号
	//   - 主 goroutine 用 select 同时监听 done 和 time.After
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	timer := time.NewTimer(waitTime)
	defer timer.Stop()
	select {
	case <-done:
		logger.Sugar.Info("协程池已经优雅关闭",
			zap.Int64("total_submitted", p.totalSubmitted.Load()),
			zap.Int64("total_completed", p.totalCompleted.Load()),
			zap.Int64("total_panic", p.totalPanic.Load()))
		return nil
	case <-timer.C:
		logger.Sugar.Warn("协程池超时，部分worker可能还在运行",
			zap.Int32("remaining_workers", p.workerNum.Load()))
		return errors.New("close pool timeout")
	}
}

// SubmitWithRetry 提交任务并在池满时重试
func SubmitWithRetry(ctx context.Context, taskFunc func(ctx context.Context), retryTimes int, interval time.Duration) error {
	if retryTimes < 0 {
		retryTimes = 0
	}

	traceID := trace.GetTraceID(ctx)

	for i := 0; i <= retryTimes; i++ {
		err := GlobalPool.Submit(ctx, taskFunc)
		if err == nil {
			return nil
		}

		// 只有池满才值得重试，其他错误直接返回
		if err != ErrPoolFull {
			logger.Sugar.Error("submit task failed (non-retryable)",
				zap.String("trace_id", traceID),
				zap.Error(err))
			return err
		}

		if i == retryTimes {
			logger.Sugar.Warn("submit task failed after all retries",
				zap.Int("total_attempts", retryTimes+1),
				zap.String("trace_id", traceID))
			return err
		}

		// 【八股：time.Sleep vs ticker vs timer】
		// 简单的一次性等待用 time.Sleep 即可，不需要 timer/ticker。
		// timer 适合需要 select 组合或需要 Stop/Reset 的场景。
		// ticker 适合周期性重复执行的场景。
		time.Sleep(interval)

		logger.Sugar.Warn("retrying submit task",
			zap.Int("attempt", i+1),
			zap.Int("remaining", retryTimes-i),
			zap.String("trace_id", traceID))
	}
	return ErrPoolFull
}

// GetPoolStats 获取池状态（监控/排查用）
func (p *GoroutinePool) GetPoolStats() map[string]interface{} {
	return map[string]interface{}{
		"core_workers":    p.coreWorkers,
		"max_workers":     p.maxWorkers,
		"current_workers": p.workerNum.Load(),
		"queue_length":    len(p.taskQueue),
		"queue_capacity":  cap(p.taskQueue),
		"total_submitted": p.totalSubmitted.Load(),
		"total_completed": p.totalCompleted.Load(),
		"total_panic":     p.totalPanic.Load(),
		"is_stopped":      p.stopped.Load(),
	}
}
