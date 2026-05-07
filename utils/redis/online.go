// 文件目的：追踪并记录哪些用户当前正连接在服务器上。
// 在线状态管理：基于 Redis String + TTL
//
// 【八股：在线状态的几种实现方案】
//
// 1. Redis String + TTL（本项目采用）
// 存储： 只要用户在线，就在 Redis 里存一个 Key（比如 im:online:张三）。SET im:online:{userID} "nodeID" EX 90
// 自动清除： 给这个 Key 设置一个 90 秒的寿命。
// 续命： 用户只要还连着，每隔一段时间（心跳）就重新刷新这个 90 秒，证明自己还活着。
// 离线： 如果用户主动断开，删除 Key；如果用户异常掉线（比如没电了），90 秒后 Redis 会自动删除 Key，系统就知道他离线了。
//    优点：简单高效，MGET 批量查询。
//    缺点：TTL 到期前有短暂的"假在线"。
//
// 2. Redis Bitmap
//    SETBIT im:online:bitmap {userID_hash} 1
//    适合海量用户（百万级），节省内存。
//    缺点：需要将 userID 映射为数字偏移量。
//
// 3. Redis Set
//    SADD im:online:set {userID}
//    适合需要遍历所有在线用户的场景。
//    缺点：没有自动过期，需要手动清理。
//
// 本项目选择方案 1：IM 场景下查询都是"查某几个好友是否在线"（MGET），
// String + TTL 最简单直接。

package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/Caodongying/dongdong-IM/utils/logger"
	"go.uber.org/zap"
)

const (
	onlineKeyTemplate = "im:online:%s" // im:online:{userID}
	onlineTTL         = 90 * time.Second // 心跳间隔（54s）的约 1.5 倍
)

// SetOnline 标记用户在线（WebSocket 连接成功时调用）。调用时机：当用户登录/建立连接成功时
func SetOnline(ctx context.Context, userID string) error {
	key := fmt.Sprintf(onlineKeyTemplate, userID)
	return RDB.Set(ctx, key, "1", onlineTTL).Err()
}

// RefreshOnline 刷新在线状态 TTL（收到心跳 Pong 时调用）。调用时机：当收到客户端发来的心跳包（Heartbeat/Ping）时
func RefreshOnline(ctx context.Context, userID string) error {
	key := fmt.Sprintf(onlineKeyTemplate, userID)
	return RDB.Expire(ctx, key, onlineTTL).Err()
}

// SetOffline 标记用户离线（WebSocket 断开时调用）。调用时机：当用户主动退出或连接断开时
func SetOffline(ctx context.Context, userID string) error {
	key := fmt.Sprintf(onlineKeyTemplate, userID)
	return RDB.Del(ctx, key).Err()
}

// IsOnline 检查单个用户是否在线
func IsOnline(ctx context.Context, userID string) bool {
	key := fmt.Sprintf(onlineKeyTemplate, userID)
	val, err := RDB.Exists(ctx, key).Result()
	return err == nil && val > 0
}

// BatchGetOnline 批量查询用户在线状态
//
// 【八股：Redis MGET 批量查询】
// MGET 一次发送多个 key 的查询，Redis 返回对应的 value 列表。
// 相比循环调用 GET，MGET 只需一次 RTT，性能提升 N 倍。
// 时间复杂度 O(N)，N 是 key 的数量。
func BatchGetOnline(ctx context.Context, userIDs []string) map[string]bool {
	if len(userIDs) == 0 {
		return nil
	}

	// 构建 key 列表
	keys := make([]string, len(userIDs))
	for i, id := range userIDs {
		keys[i] = fmt.Sprintf(onlineKeyTemplate, id)
	}

	// 【八股：MGET 返回值处理】
	// MGET 返回 []interface{}，每个元素是 string 或 nil：
	//   - string：key 存在，返回 value
	//   - nil：key 不存在（用户离线或 TTL 过期）
	results, err := RDB.MGet(ctx, keys...).Result()
	if err != nil {
		logger.Sugar.Error("批量查询在线状态失败", zap.Error(err))
		return nil
	}

	onlineMap := make(map[string]bool, len(userIDs))
	for i, result := range results {
		onlineMap[userIDs[i]] = result != nil
	}

	return onlineMap
}
