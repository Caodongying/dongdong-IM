// 本文件用一个统一的函数GetWithProtection同时解决三个问题：缓存击穿，缓存穿透，缓存雪崩

package redis

import (
	"context"
	"math/rand"
	"time"

	"github.com/Caodongying/dongdong-IM/utils/logger"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)


var sfGroup singleflight.Group

// 空值缓存的标记（区分"缓存 miss"和"缓存了空值"）
const emptyCacheValue = "<NIL>"

// 空值缓存的 TTL（较短，防止数据库已有新数据但缓存仍返回空）
const emptyCacheTTL = 60 * time.Second

// GetWithProtection 带三件套防护的缓存查询
//
// 使用方式：
//
//	result, err := redis.GetWithProtection(ctx, "im:user:id:xxx", 24*time.Hour, func() (string, error) {
//	    // 这里是查数据库的逻辑
//	    return queryFromDB()
//	})
//
// 参数：
//   - key: 缓存 key
//   - baseTTL: 基础过期时间（会自动加随机抖动防雪崩）
//   - loader: 缓存 miss 时的数据加载函数（通常是查数据库）
func GetWithProtection(ctx context.Context, key string, baseTTL time.Duration, loader func() (string, error)) (string, error) {
	// 第一步：查缓存
	val, err := RDB.Get(ctx, key).Result()
	if err == nil {
		// 缓存命中
		if val == emptyCacheValue {
			// 【防穿透】命中了空值缓存，说明数据库中确实没有这个数据
			return "", nil
		}
		return val, nil
	}

	// 第二步：缓存 miss，使用 singleflight 防击穿- 同一个 key 的多个并发请求，只有一个会执行 loader，其他请求等待并共享结果。
	// 情况2：err != nil
	// 这里包括：
	//   - key从未存在过
	//   - key已过期（被Redis自动删除，因为Redis 的 TTL 过期机制是自动删除 key，而不是返回一个特殊值。当 key 过期后，它就像从未存在过一样。）
	//   - Redis故障
	//
	result, singleErr, _ := sfGroup.Do(key, func() (any, error) {
		// 双重检查：在获得锁后再查一次缓存（可能另一个请求已经写入了）
		if val, err := RDB.Get(ctx, key).Result(); err == nil {
			if val == emptyCacheValue {
				return "", nil
			}
			return val, nil
		}

		// 执行 loader（查数据库）
		dbVal, dbErr := loader()
		if dbErr != nil {
			return "", dbErr
		}

		if dbVal == "" {
			// 【防穿透】数据库中也没有，缓存空值
			// 用较短的 TTL，给数据库新写入数据留出空间
			if err := RDB.Set(ctx, key, emptyCacheValue, emptyCacheTTL).Err(); err != nil {
				logger.Sugar.Warn("缓存空值写入失败",
					zap.String("key", key),
					zap.Error(err))
			}
			return "", nil
		}

		// 写入缓存
		// 【防雪崩】TTL 加随机抖动，分散过期时间
		// 在 baseTTL 的基础上加 0~10% 的随机偏移
		jitter := time.Duration(rand.Int63n(int64(baseTTL) / 10))
		ttl := baseTTL + jitter
		if err := RDB.Set(ctx, key, dbVal, ttl).Err(); err != nil {
			logger.Sugar.Warn("缓存写入失败",
				zap.String("key", key),
				zap.Error(err))
		}

		return dbVal, nil
	})

	if singleErr != nil {
		return "", singleErr
	}

	return result.(string), nil
}

// ************************** 八股 **************************
// *********************************************************

// 缓存防护三件套：防穿透、防雪崩、防击穿

// 【八股：缓存穿透 vs 缓存雪崩 vs 缓存击穿】

// 1. 缓存穿透（Cache Penetration） 缓存里没有，数据库里也没有 → 请求每次都走到DB → 缓存形同虚设

//    场景：查询一个数据库中不存在的数据（如 ID=-1）
//    问题：每次请求都穿透缓存直达数据库，缓存形同虚设
//    方案：空值缓存（缓存 null 值，这个也能应对缓存击穿）+ 布隆过滤器
//
// 2. 缓存雪崩（Cache Avalanche） 大量key同时过期 → 一瞬间缓存大量miss → DB压力爆炸
//    场景：大量 key 在同一时间过期
//    问题：过期瞬间大量请求涌入数据库，可能打崩
//    方案：TTL 加随机抖动，分散过期时间
//
// 3. 缓存击穿（Cache Breakdown / Hot Key Expiration） 一个热点key过期 → 瞬间大量请求同时miss → 把这个热点key对应的DB打爆
//    场景：某个热点 key 过期的瞬间，大量并发请求同时查询
//    问题：这些请求全部穿透到数据库，产生瞬间高压
//    方案：singleflight（同一个 key 只放一个请求去查 DB，其他等待共享结果）


// 问题	   解决方案						记忆点
// 穿透	   空值缓存 / 布隆过滤器		  不存在的东西，放个占位符
// 雪崩	   随机TTL					    让过期时间错开，别一起倒
// 击穿	   singleflight / 互斥锁		一个人去查，其他人等着

// 全局 singleflight group
// 【八股：singleflight 原理】
// singleflight.Group 维护一个 map[key]*call，
// 第一个请求进来创建 call 并实际执行函数，
// 后续相同 key 的请求直接等待第一个 call 的结果。
// 底层用 sync.Mutex + sync.WaitGroup 实现。

// 想象一个场景：微博热搜榜的缓存过期了，瞬间100万个请求同时来查这个数据。
// 没有singleflight：100万个请求全部穿透到数据库 → 数据库挂了
// 有singleflight：100万个请求中，只有第1个去查数据库，剩下999,999个等着第1个回来，然后共享结果 → 数据库只收到1个请求

// var sfGroup singleflight.Group

// // 100个并发请求调用这段代码
// result, err, _ := sfGroup.Do("hot_key", func() (interface{}, error) {
//     // 这个函数只会执行1次
//     return queryDB()
// })


// loader 参数的设计本质是：
// 		模式：策略模式 + 模板方法模式
// 		目的：把"怎么查数据"这个可变部分抽离出来
// 		好处：一份缓存防护代码，复用于所有需要缓存的地方
// 其实就是，GetWithProtection 控制了"何时"执行数据库查询，而调用方只需要关心"如何"查询
// 不然就会是这么多函数(每个场景写一个专用函数):GetUserWithProtection, GetOrderWithProtection...
// 或者用interface，更复杂。