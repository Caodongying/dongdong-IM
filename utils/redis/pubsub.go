// Redis Pub/Sub 跨节点消息推送(只针对双方用户都在线)

// 业务场景：用户A向用户B发送消息Msg。负责处理websocket消息的服务器可能有很多台。
// 用户A保存在节点1，用户B保存在节点2.这里“保存”的意思是用户信息保存在本地Hub结构的clients sync.Map里
// 因为在本地服务器找不到用户B，所以节点1需要将消息扩散，寻找用户B所在的服务器
// 这个扩散的媒介是Redis channel。
// 每个节点都订阅这个频道，有一个专门的订阅处理函数，会读取频道里的数据然后按照传入
// 的onMessage函数进行处理。如果要发送数据到频道，就调用封装好的sdk。

// 【八股：为什么 IM 需要跨节点推送？】
// 单机 Hub 只能管理本节点的 WebSocket 连接。当部署多个实例时，用户 A 连在节点 1，用户 B 连在节点 2，
// A 发消息给 B，节点 1 的 Hub 找不到 B 的连接。

// 解决方案：Redis Pub/Sub 做消息总线
//   1. 节点 1 发现 B 不在本地 Hub → 将消息 PUBLISH 到 Redis channel
//   2. 所有节点都 SUBSCRIBE 同一个 channel
//   3. 节点 2 收到消息 → 在本地 Hub 找到 B 的连接 → 推送

// 【八股：Pub/Sub vs Kafka】
// - Pub/Sub：消息不持久化，订阅者断开后消息丢失，适合实时推送
// - Kafka：消息持久化，有 offset 回溯能力，适合消息落库等需要可靠性的场景
// IM 场景中两者互补：Pub/Sub 做实时推送，Kafka 做异步落库

package redis

import (
	"context"
	"encoding/json"

	"github.com/Caodongying/dongdong-IM/utils/logger"
	"go.uber.org/zap"
)

const (
	// 跨节点消息推送 channel
	// 【八股：Redis Pub/Sub channel 设计】
	// 用固定 channel 名 + 消息体内包含 target userID，
	// 每个节点收到后检查 target 是否在本节点，不在则忽略。
	crossNodeChannel = "im:cross_node:push"
)

// CrossNodeMessage 跨节点消息结构
type CrossNodeMessage struct {
	TargetUserID string `json:"target_user_id"` // 目标用户 ID
	Data         []byte `json:"data"`           // 序列化后的 WebSocket 消息
}

// PublishCrossNode 发布跨节点消息
// 当本地 Hub 找不到目标用户时调用
func PublishCrossNode(ctx context.Context, targetUserID string, data []byte) error {
	msg := CrossNodeMessage{
		TargetUserID: targetUserID,
		Data:         data,
	}
	// Marshal和UnMarshal是Go结构体和网络传输格式之间的互相转换
	payload, err := json.Marshal(msg) // 把 Go 对象转成 JSON 字节
	if err != nil {
		return err
	}

	// 【八股：Redis PUBLISH 命令】
	// PUBLISH channel message
	// 将消息发送给所有订阅了该 channel 的客户端
	// 返回值是收到消息的订阅者数量（可用于监控）
	return RDB.Publish(ctx, crossNodeChannel, payload).Err()
}


// SubscribeCrossNode 订阅跨节点消息
// 传入回调函数，当收到其他节点推送的消息时调用
//
// 【八股：Redis Pub/Sub 的 Go 实现模式】
// go-redis 的 Subscribe 返回 *PubSub 对象，通过 Channel() 获取消息 channel，
// 然后用 for range 消费。这是 Go 中消费 Pub/Sub 的标准模式。

// 链路：
//   Redis Server
//      ↓ TCP socket
//   go-redis PubSub 连接
//      ↓ 后台 goroutine 读取 socket
//   go-redis 解析 RESP 消息
//      ↓
//   写入 Go channel ch
//      ↓
//   你的 for/select 读取 ch

func SubscribeCrossNode(ctx context.Context, onMessage func(targetUserID string, data []byte)) {
	// 【八股：Redis SUBSCRIBE 命令】
	// SUBSCRIBE channel：订阅一个 channel，之后该连接进入订阅模式，
	// 只能执行 SUBSCRIBE/UNSUBSCRIBE/PSUBSCRIBE/PUNSUBSCRIBE 命令。
	// go-redis 内部用独立连接实现，不影响其他 Redis 操作。

	// 不需要对这个redis channel进行初始化。因为 Redis Pub/Sub channel 不是 Redis 的持久化数据结构，更像 Redis server 内部维护的一张“订阅关系表”。
	// 内部可以粗略理解成 Redis 有个运行时 map：map[channelName][]subscriberConnection
	pubsub := RDB.Subscribe(ctx, crossNodeChannel)

	// 等待订阅成功确认
	_, err := pubsub.Receive(ctx)
	if err != nil {
		logger.Sugar.Error("Redis Pub/Sub 订阅失败", zap.Error(err))
		return
	}

	logger.Sugar.Info("Redis Pub/Sub 订阅成功",
		zap.String("channel", crossNodeChannel))

	// 获取消息 channel（有缓冲，默认 100）
	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			pubsub.Close()
			logger.Sugar.Info("Redis Pub/Sub 订阅已关闭")
			return
		case redisMsg, ok := <-ch:
			if !ok {
				return
			}

			var msg CrossNodeMessage
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil { // 把 JSON 字节解析回 Go 对象
				logger.Sugar.Warn("跨节点消息反序列化失败", zap.Error(err))
				continue
			}

			onMessage(msg.TargetUserID, msg.Data)
		}
	}
}
