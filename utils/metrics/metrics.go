// Prometheus 指标定义与注册
//
// 【八股：可观测性三大支柱】
// 1. Metrics（指标）：数值型时间序列，用于监控告警（Prometheus + Grafana）
// 2. Logging（日志）：事件流，用于排查问题（ELK / Loki）
// 3. Tracing（链路追踪）：请求的完整调用链（Jaeger / Zipkin）
//
// 本模块实现 Metrics 部分，暴露 /metrics 端点供 Prometheus 抓取。
//
// 【八股：Prometheus 的四种指标类型】
// 1. Counter：只增不减的计数器（如请求总数、错误总数）
// 2. Gauge：可增可减的仪表盘（如在线人数、goroutine 数量）
// 3. Histogram：直方图，统计值的分布（如请求延迟的 P50/P99）
// 4. Summary：类似 Histogram，但在客户端计算分位数

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ======== WebSocket 指标 ========

	// WSOnlineGauge 当前在线连接数（Gauge：可增可减）
	WSOnlineGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "dongdong_im",
		Subsystem: "ws",
		Name:      "online_connections",
		Help:      "当前 WebSocket 在线连接数",
	})

	// WSMessageTotal 消息处理总数（Counter：只增不减）
	// 按 type 标签区分单聊/群聊/ACK 等
	WSMessageTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "dongdong_im",
		Subsystem: "ws",
		Name:      "messages_total",
		Help:      "WebSocket 消息处理总数",
	}, []string{"type"}) // type: single_chat, group_chat, ack

	// ======== gRPC 指标 ========

	// GRPCRequestTotal gRPC 请求总数
	GRPCRequestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "dongdong_im",
		Subsystem: "grpc",
		Name:      "requests_total",
		Help:      "gRPC 请求总数",
	}, []string{"method", "code"})

	// GRPCRequestDuration gRPC 请求延迟（Histogram）
	// 【八股：Histogram 的 bucket 设计】
	// bucket 定义了延迟区间，Prometheus 会统计落在每个区间的请求数。
	// 一般按业务 SLA 设计：5ms/10ms/25ms/50ms/100ms/250ms/500ms/1s/2.5s
	// P99 < 100ms 是大多数 IM 系统的 SLA。
	GRPCRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "dongdong_im",
		Subsystem: "grpc",
		Name:      "request_duration_seconds",
		Help:      "gRPC 请求延迟（秒）",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
	}, []string{"method"})

	// ======== 业务指标 ========

	// MessagePersistTotal 消息持久化总数
	MessagePersistTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "dongdong_im",
		Subsystem: "message",
		Name:      "persist_total",
		Help:      "消息持久化总数",
	}, []string{"status"}) // status: success, fail

	// GoroutinePoolGauge 协程池活跃 worker 数
	GoroutinePoolGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "dongdong_im",
		Subsystem: "pool",
		Name:      "active_workers",
		Help:      "协程池当前活跃 worker 数",
	})

	// RateLimitRejectTotal 限流拒绝总数
	RateLimitRejectTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "dongdong_im",
		Subsystem: "ratelimit",
		Name:      "reject_total",
		Help:      "限流拒绝请求总数",
	}, []string{"level"}) // level: global, ip
)
