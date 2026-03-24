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

	ErrNilTask = errors.New("空任务函数")
	// 错误信息是固定的用 errors.New，需要带变量用 fmt.Errorf
)

type GoroutionPool struct {
	taskQueue chan func() // 任务队列
	coreWorkers int32 // 核心worker
	maxWorkers int32 // 最大worker数（核心+临时）
	idleTimeout time.Duration // 临时协程空闲超时时间

	// 当前运行中的worker数量
	workerNum atomic.Int32 // 这是值是在运行中会被改变的，为了避免并发冲突，用atomic包

	// 与协程池关闭有关的值
	wg sync.WaitGroup
	stopped atomic.Bool
}

// 新建协程池
func NewPool(coreWorkers, maxWorkers, queueSize int, idleTimeout time.Duration) (*GoroutinePool, error) {
	// 参数校验
	if coreWorkers <= 0 || maxWorkers < coreWorkers || queueSize <= 0 {
		return nil, fmt.Errorf("invalid pool config: core=%d, max=%d, queue=%d",
			coreWorkers, maxWorkers, queueSize)
	}

	// 实例化协程池
	p := &GoroutinePool{
		coreWorkers: int32(coreWorkers),
		maxWorkers:  int32(maxWorkers),
		idleTimeout: idleTimeout,
		taskQueue: make(chan func(), queueSize),
	}

	// 启动核心worker
	for i := 0; i < coreWorkers; i++ {
		p.workerNum.Add(1)
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			defer p.workerNum.Add(-1) // 为什么这里不用CAS，不会有并发冲突吗，比如同时对5减一
			for task := range p.taskQueue {
				// 执行任务
				p.runTask(task)
			}
		}() // 不能省略最后的()，不然就不是调用函数了
	}

	return p, nil
}

// 在运行task时，包含panic捕捉
func (p *GoroutinePool) runTask(task func()) {
	// 捕捉panic，防止协程池直接crashes
	defer func(){
		if r := recover(); r!= nil {
			logger.Sugar.Error("task panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	task()
}


// 初始化协程池
func Init() error {
	config := config.Viper.GetConfig().Pool
	p, err := NewPool(config.CoreSize, config.MaxSize, config.QueueSize, time.Duration(config.IdleTimeout)*time.Second)
	if err != nil {
		logger.Sugar.Error("无法初始化协程池", zap.Error(err))
		return err
	}

	GlobalPool = p
	logger.Sugar.Info("协程池初始化成功",
		zap.Int("核心workers", config.CoreSize),
		zap.Int("最大workers", config.MaxSize),
		zap.Int("队列大小", config.QueueSize),
		zap.Int("临时worker空闲超时时间", config.IdleTimeout),
	)

	return nil

}

// 单次提交任务
func (p *GoroutinePool) Submit(ctx context.Context, taskFunc func(ctx context.Context)) error {
	// 参数校验
	if task == nil {
		return ErrNilTask
	}

	// 判断协程池是否已经关闭
	if p.stopped.Load(){
		return ErrPoolStopped
	}

	// 将带context的task函数封装为不带参数的闭包，为入队做准备
	task := func() {
		// 判断context是否到期
		select {
		case <- ctx.Done():
			logger.Sugar.Warn("任务跳过，context取消", zap.String("trace_id", trace.GetTraceID(ctx)), zap.Error(ctx.Err()))
			return
		default:
			// 执行task
			taskFunc(ctx)
		}
	}

	// 三级提交策略
	// 1. 如果任务队列没满，尝试直接入队
	select {
	case p.taskQueue <- task:
		return nil // 提交成功
	default:
	}

	// 2. 如果任务队列满了，但是还有空闲的临时worker，启用
	for {
		current := p.workerNum.Load()
		if current >= p.maxWorkers{
			break // 已经到达上限，无法扩容
		}

		// 更新workerNum
		if p.workerNum.CompareAndSwap(current, current+1){ // CAS成功
			// 启用临时worker
			go func(){
				defer p.wg.Done()
				defer p.workerNum.Add(-1)

				p.wg.Add(1) // --
				p.workerNum.Add(1) // --

				// 设置超时
				idleTimer := time.NewTimer(p.idleTimeout)
				defer idleTimer.Stop()

				// 从taskQueue里拿任务
				for {
					select {
					case task, ok := <- p.taskQueue:
						if !ok {
							// 任务队列已经为空，channel已经关闭
							return
						}
						p.runTask(task)

						// 重置超时器
						if !idleTimer.Stop() {
							select {
							case <- idleTimer.C:
							default:
							}
						}
						idleTimer.Reset(p.idleTimeout)
					case <- idleTimer.C:
						logger.Sugar.Debug("空闲超时，临时 worker 退出",
							zap.Int32("current_workers", p.workerNum.Load()))
						return // 超时了，协程退出
					}
				}
			}()
			// 扩容后再次尝试入队
			select{
			case p.taskQueue <- task:
				return nil
			default:
				// 扩容后，任务队列依然满
			}

			break
		}

		// CAS失败，继续自旋重试

	}


	// 3. 最后查看一下taskQueue任务队列是否有新的空位子
	// 再次检查有必要，因为临时worker的加入，增加了吞吐量
	// 没有临时worker了，返回错误

	select{
	case p.taskQueue <- task:
		return nil
	default:
		return ErrNilTask
	}


}

// 带重试的提交
func (p *GoroutinePool) SubmitWithRetry() {

}

// 关闭协程池
func (p *GoroutinePool) Close() {

}