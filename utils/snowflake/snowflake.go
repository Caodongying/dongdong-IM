// Snowflake ID 生成器：生成全局唯一、趋势递增的 64 位 ID
//
// 【八股：为什么不用 MySQL 自增 ID？】
// 1. 自增 ID 在分库分表场景下无法保证全局唯一
// 2. 自增 ID 暴露业务量（用户能猜出你有多少条数据）
// 3. 自增 ID 在高并发插入时存在锁竞争（InnoDB 的 AUTO_INCREMENT 锁）
//
// 【八股：为什么不用 UUID？】
// UUID 是 128 位字符串，存储和索引效率低。
// 更关键的是 UUID 完全随机，不趋势递增，导致 B+ 树索引大量页分裂，
// 写入性能远不如 Snowflake 这种趋势递增的 ID。
//
// 【八股：Snowflake ID 结构（64 bit）】
//
//	| 1 bit 符号位 | 41 bit 时间戳 | 10 bit 机器ID | 12 bit 序列号 |
//
// - 符号位：固定为 0（正数），否则就表示负数，对InnoDB的B+树索引排序有性能影响
// - 时间戳：相对于自定义 epoch 的毫秒数，可用约 69 年
// - 机器 ID：支持 1024 个节点（0-1023）
// - 序列号：同一毫秒内的自增序列，支持每毫秒 4096 个 ID
//
// 优势：趋势递增（利于 B+ 树）、全局唯一、不依赖数据库、高性能
// 劣势：1. 时钟回拨： 极度依赖系统时间。如果机器时间回拨，会导致 ID 重复（工业实现通常会检查回拨并抛异常或等待）。
// 		2. 机器 ID 分配： 在大规模容器化部署（如 K8s）中，如何自动分配 10 bits 的机器 ID 是个麻烦。工业界主要通过**外部存储（注册中心）**来自动管理这 10 bits 的机器 ID

package snowflake

import (
	"errors"
	"sync"
	"time"

	"github.com/Caodongying/dongdong-IM/utils/config"
	"github.com/Caodongying/dongdong-IM/utils/logger"
	"go.uber.org/zap"
)

// Const 定义“算法规则”： Snowflake 算法的位数分配（比如 10 位机器码、12 位序列号）
// 通常是全局统一的。一旦确定，不希望在运行时被修改。如果放进 struct，
// 就暗示了这些值可以在运行时改变，这会增加维护心智负担。
const (
	// 自定义 epoch：2026-01-01 00:00:00 UTC
	// 选择一个较近的时间点作为起始，可以让 41 bit 时间戳用更久
	epoch int64 = 1767225600000

	// 各部分的位数
	machineBits  = 10 // 机器 ID 占 10 位
	sequenceBits = 12 // 序列号占 12 位

	// 最大值（通过位运算计算）
	// -1 的二进制是全 1（补码），左移 N 位后再取反，得到 N 位全 1 的数
	// 例：-1 ^ (-1 << 10) = 0x3FF = 1023
	maxMachineID = -1 ^ (-1 << machineBits)  // 1023
	maxSequence  = -1 ^ (-1 << sequenceBits) // 4095

	// 左移位数
	machineShift  = sequenceBits                 // 机器 ID 左移 12 位
	timestampShift = sequenceBits + machineBits  // 时间戳左移 22 位
)

var (
	ErrClockBackward = errors.New("时钟回拨，拒绝生成 ID")
	ErrInvalidMachine = errors.New("机器 ID 超出范围 (0-1023)")
)

// Generator Snowflake ID 生成器
// Struct 应该保存“状态”
// machineID、sequence、lastTime 是会随着时间或实例不同而变化的，它们定义了一个生成器的当前运行情况。
type Generator struct {
	mu        sync.Mutex // 保护以下字段的并发安全
	machineID int64      // 机器 ID
	sequence  int64      // 当前毫秒内的序列号
	lastTime  int64      // 上次生成 ID 的时间戳（毫秒）
}

// 全局生成器实例
var Global *Generator

// Init 初始化全局 Snowflake 生成器
func Init() error {
	machineID := config.Viper.GetConfig().Snowflake.MachineID
	g, err := NewGenerator(machineID)
	if err != nil {
		return err
	}
	Global = g
	logger.Sugar.Info("Snowflake ID 生成器初始化成功",
		zap.Int64("machine_id", machineID))
	return nil
}

// NewGenerator 创建新的 Snowflake 生成器
func NewGenerator(machineID int64) (*Generator, error) {
	if machineID < 0 || machineID > maxMachineID {
		return nil, ErrInvalidMachine
	}
	return &Generator{
		machineID: machineID,
	}, nil
}

// NextID 生成下一个 Snowflake ID
//
// 【八股：为什么用 Mutex 而不是 atomic/CAS？】
// ID 生成涉及多个变量的联动操作（时间戳 + 序列号 + 上次时间），
// 这些变量之间有依赖关系，需要保证一致性，单个 atomic 操作无法做到。
// 而且 ID 生成虽然是热点操作，但 Mutex 在无竞争时只是一次 CAS，
// 有竞争时也只是微秒级，对 IM 场景完全够用。
func (g *Generator) NextID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UnixMilli()

	// 【八股：时钟回拨问题】
	// 如果服务器时间被人为调回（NTP 同步、手动修改）或者系统有什么问题，
	// 可能导致生成的 ID 重复（同一毫秒 + 同一序列号）。
	// 处理策略：
	//   - 小幅回拨（<5ms）：等待到上次时间，容忍短暂阻塞
	//   - 大幅回拨：直接报错，拒绝生成
	if now < g.lastTime {
		offset := g.lastTime - now
		if offset <= 5 {
			// 小幅回拨，等待追上
			time.Sleep(time.Duration(offset) * time.Millisecond)
			now = time.Now().UnixMilli()
		}
		if now < g.lastTime {
			return 0, ErrClockBackward
		}
	}

	if now == g.lastTime {
		// 同一毫秒内，序列号自增
		g.sequence = (g.sequence + 1) & maxSequence
		if g.sequence == 0 {
			// 序列号溢出（当前毫秒已生成 4096 个 ID），等待下一毫秒
			now = g.waitNextMilli(now)
		}
	} else {
		// 新的毫秒，序列号重置
		g.sequence = 0
	}

	g.lastTime = now

	// 【八股：位运算组装 ID】
	// 通过左移 + 或运算将各部分拼接成一个 64 位整数：
	// | 时间戳部分 | 机器ID部分 | 序列号部分 |
	id := ((now - epoch) << timestampShift) |
		(g.machineID << machineShift) |
		g.sequence

	return id, nil
}

// waitNextMilli 阻塞等待直到下一毫秒
func (g *Generator) waitNextMilli(lastTime int64) int64 {
	now := time.Now().UnixMilli()
	for now <= lastTime {
		// 短暂自旋等待，不用 time.Sleep 因为只等不到 1ms
		now = time.Now().UnixMilli()
	}
	return now
}
