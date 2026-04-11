// 限流器要解决的问题：有人恶意刷接口，要把服务器搞崩溃。
// 核心逻辑根本不在这个文件里，而是调用了 Go 官方库 golang.org/x/time/rate 的 Allow() 方法。

package ratelimit

import (
	"sync"
	"time"

	"github.com/Caodongying/dongdong-IM/utils/config"
	"github.com/Caodongying/dongdong-IM/utils/logger"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// RateLimiter 两级限流器
type RateLimiter struct {
	// 全局限流器：限制整个服务的 QPS
	global *rate.Limiter

	// IP 维度限流器：防止单个 IP 刷接口
	ipLimiters sync.Map // key: IP string, value: *ipLimiterEntry
}

// ipLimiterEntry 包含限流器和最后访问时间（用于过期清理）
type ipLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// AllowGlobal 全局限流检查
func (rl *RateLimiter) AllowGlobal() bool {
	return rl.global.Allow()
}

// AllowIP 单 IP 限流检查
func (rl *RateLimiter) AllowIP(ip string) bool {
	// 获取或创建该 IP 的限流器
	entry, _ := rl.ipLimiters.LoadOrStore(ip, &ipLimiterEntry{
		// 单个 IP 限制：每秒 100 请求，突发 200
		limiter:  rate.NewLimiter(100, 200),
		lastSeen: time.Now(),
	})

	e := entry.(*ipLimiterEntry)
	e.lastSeen = time.Now()

	return e.limiter.Allow()
}

// cleanupLoop 定时清理过期 IP 限流器
// 为什么要清理？因为恶意用户可能用大量不同 IP 攻击，每个 IP 都会在 sync.Map 里留下一个限流器，不清理会内存泄漏。
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		expired := 0
		rl.ipLimiters.Range(func(key, value any) bool {
			entry := value.(*ipLimiterEntry)
			if now.Sub(entry.lastSeen) > 10*time.Minute {
				rl.ipLimiters.Delete(key)
				expired++
			}
			return true
		})
		if expired > 0 {
			logger.Sugar.Debug("清理过期 IP 限流器",
				zap.Int("expired_count", expired))
		}
	}
}

// 全局限流器实例
var Global *RateLimiter

// Init 初始化限流器
func Init() {
	cfg := config.Viper.GetConfig().Limit
	Global = &RateLimiter{
		// 【八股：rate.NewLimiter(r, b) 参数含义】
		// r = rate：每秒生成的令牌数（QPS 上限）
		// b = burst：令牌桶容量（允许的最大突发量）
		// 例如 rate=1000, burst=2000：平均 QPS 限制 1000，允许瞬间突发 2000
		// Rate 控制平均速率（长期稳定）  Burst 允许突发流量（短期尖峰）
		global: rate.NewLimiter(rate.Limit(cfg.Rate), cfg.Burst),
		// sync 包的并发原语都是零值可用，map/slice/指针需要初始化。
	}

	// 启动定时清理过期的 IP 限流器（防止内存泄漏）
	go Global.cleanupLoop()

	logger.Sugar.Info("限流器初始化成功",
		zap.Int("rate", cfg.Rate),
		zap.Int("burst", cfg.Burst))
}

// ******************** 八股总结 ********************
// 限流器：令牌桶算法实现，支持全局限流 + IP 维度限流
// 【八股：常见限流算法对比】
//
// 1. 固定窗口计数器：每个时间窗口一个计数器，超过阈值拒绝。
//    缺点：窗口边界处可能突发 2 倍流量（前一窗口末尾 + 后一窗口开头）。
//
// 2. 滑动窗口计数器：将固定窗口细分为多个小窗口，滑动统计。
//    改善了边界突发问题，但实现更复杂。
//
// 3. 漏桶（Leaky Bucket）：请求进入桶，以固定速率流出。
//    优点：输出速率恒定。缺点：无法应对合理突发（即使空闲很久，也只能匀速处理）。
//
// 4. 令牌桶（Token Bucket）：以固定速率向桶中添加令牌，请求消耗令牌。
//    优点：允许一定突发（桶满时可以一次消耗多个令牌），同时限制平均速率。
//    Go 标准库扩展 golang.org/x/time/rate 就是令牌桶实现。
//
// 本项目选择令牌桶：允许合理突发（IM 消息天然有突发性），同时限制平均 QPS。

// 请求进来
//     ↓
// 全局限流（整个服务总的 QPS 限制）
//     ↓ 通过
// IP 限流（单个 IP 的 QPS 限制）
//     ↓ 通过
// 放行

// 【八股：sync.Map 的适用场景】
// IP 限流器的读写模式是：不同 goroutine 操作不同 key（不同 IP），
// 很少有多个 goroutine 同时操作同一个 key。
// 这正是 sync.Map 的最佳场景（无竞争的并发读写）。

	// 【八股：rate.Limiter.Allow() 的实现原理】
	// Allow() 尝试消耗一个令牌：
	//   - 桶中有令牌 → 消耗一个，返回 true
	//   - 桶中无令牌 → 返回 false（不等待）
	// 具体的内部逻辑——
		// 当前时间 - 上次令牌生成时间 = 间隔
		// 新令牌数 = 间隔 × 生成速率（Rate）

		// 桶中令牌数 = min(桶中令牌数 + 新令牌数, Burst)

		// if 桶中令牌数 >= 1 {
		//     桶中令牌数--
		//     return true  // 放行
		// }
		// return false  // 拒绝

	// 底层用 atomic 操作 + 时间计算，无锁高性能

// 为什么AllowIP里要更新lastSeen，全局的limiter就不用
// 核心原因：全局限流器只有一个，IP 限流器有无数个。
// 这个时间戳是给清理程序看的


