# dongdong-IM 架构设计图

## 一、系统整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                          客户端 (Client)                            │
│              移动端 / Web 端 / 桌面端 / gRPC Client                  │
└──────────┬──────────────────────────────────┬───────────────────────┘
           │ WebSocket (长连接)                │ gRPC (短连接)
           │ ws://host:8080/ws               │ host:50051
           ▼                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        接入层 (API Layer)                           │
│                                                                     │
│  ┌─────────────────────┐        ┌──────────────────────────────┐   │
│  │   WebSocket Server   │        │       gRPC Server            │   │
│  │   (net/http :8080)   │        │       (:50051)               │   │
│  │                      │        │                              │   │
│  │  /ws    → ServeWS    │        │  拦截器链 (Interceptor Chain) │   │
│  │  /metrics→Prometheus │        │  ┌────────────────────────┐  │   │
│  │  /health → 健康检查   │        │  │ Recovery (panic捕获)   │  │   │
│  │                      │        │  │ Metrics  (RED指标)     │  │   │
│  │  ┌────────────────┐  │        │  │ RateLimit(令牌桶限流)  │  │   │
│  │  │      Hub       │  │        │  │ Trace    (TraceID注入) │  │   │
│  │  │ (连接管理中心)  │  │        │  │ Auth     (JWT鉴权)    │  │   │
│  │  │  sync.Map      │  │        │  └────────────────────────┘  │   │
│  │  │  register ch   │  │        │                              │   │
│  │  │  unregister ch │  │        │  UserService                 │   │
│  │  └───────┬────────┘  │        │  FriendService               │   │
│  │          │            │        │  MessageService              │   │
│  │    ┌─────┴─────┐     │        │  GroupService                │   │
│  │    │  Client×N  │     │        │                              │   │
│  │    │ ReadPump   │     │        └──────────────────────────────┘   │
│  │    │ WritePump  │     │                                          │
│  │    └───────────┘     │                                          │
│  └─────────────────────┘                                           │
└──────────┬──────────────────────────────────┬───────────────────────┘
           │                                  │
           ▼                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        业务层 (Service Layer)                       │
│                                                                     │
│  ┌──────────┐  ┌──────────────┐  ┌──────────────┐  ┌───────────┐  │
│  │  User    │  │   Message    │  │   Friend     │  │   Group   │  │
│  │ Service  │  │   Service    │  │   Service    │  │  Service  │  │
│  │          │  │              │  │              │  │           │  │
│  │ Register │  │ SaveMessage  │  │ AddFriend    │  │ Create    │  │
│  │ Login    │  │ PushToInbox  │  │ HandleFriend │  │ Join      │  │
│  │          │  │ PullOffline  │  │ ListFriends  │  │ Leave     │  │
│  │          │  │ History      │  │ DeleteFriend │  │ Dismiss   │  │
│  │          │  │ Ack          │  │ Pending      │  │ List      │  │
│  └────┬─────┘  └──────┬───────┘  └──────┬───────┘  └─────┬─────┘  │
│       │               │                 │                 │        │
│       └───────────────┴─────────────────┴─────────────────┘        │
│                               │                                     │
└───────────────────────────────┼─────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      基础设施层 (Infrastructure)                     │
│                                                                     │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌──────────────────┐  │
│  │  MySQL   │  │  Redis   │  │ Snowflake │  │  Goroutine Pool  │  │
│  │  (GORM)  │  │          │  │ ID 生成器  │  │  (自研协程池)     │  │
│  │          │  │ Cache    │  │           │  │                  │  │
│  │ im_user  │  │ Lock     │  │ 41bit时间  │  │ Core + Temp      │  │
│  │ im_msg   │  │ Online   │  │ 10bit机器  │  │ CAS无锁扩容      │  │
│  │ im_friend│  │ Pub/Sub  │  │ 12bit序列  │  │ 三级提交策略      │  │
│  │ im_group │  │ Inbox    │  │           │  │                  │  │
│  └──────────┘  └──────────┘  └───────────┘  └──────────────────┘  │
│                                                                     │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌──────────────────┐  │
│  │   JWT    │  │  Logger  │  │ RateLimit │  │    Metrics       │  │
│  │  鉴权    │  │  (Zap)   │  │ (令牌桶)   │  │  (Prometheus)    │  │
│  └──────────┘  └──────────┘  └───────────┘  └──────────────────┘  │
│                                                                     │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐                        │
│  │  Config  │  │  Trace   │  │ ShardLock │                        │
│  │ (Viper)  │  │ TraceID  │  │ (分片锁)   │                        │
│  └──────────┘  └──────────┘  └───────────┘                        │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 二、初始化链（启动顺序）

```
config.Init()        配置加载（Viper + 热更新）
      │
      ▼
logger.Init()        结构化日志（Zap + 文件切割）
      │
      ▼
mysql.Init()         数据库连接池（GORM + 慢查询日志）
      │
      ▼
redis.Init()         Redis 连接池
      │
      ▼
jwt.Init()           JWT 密钥加载
      │
      ▼
snowflake.Init()     Snowflake ID 生成器（机器号配置）
      │
      ▼
ratelimit.Init()     令牌桶限流器（全局 + per-IP）
      │
      ▼
pool.Init()          协程池启动（Core + Temp workers）
      │
      ▼
ws.Init()            WebSocket Hub 启动 + Redis Pub/Sub 订阅
      │
      ▼
回调注入              SetOnMessageReceived / SetGetGroupMembers / SetOnGroupMessageReceived
      │
      ▼
grpc.NewServer()     gRPC server + 拦截器链 + 服务注册
      │
      ├─→ go grpcServer.Start()    gRPC 监听 :50051
      │
      └─→ go httpServer.Listen()   HTTP  监听 :8080（WS + Metrics + Health）
              │
              ▼
         等待关闭信号 (SIGTERM / SIGINT)
```

---

## 三、WebSocket 消息收发流程

### 3.1 单聊消息流

```
发送者 Alice                    服务端                         接收者 Bob
   │                              │                              │
   │  1. WS 发送消息 JSON          │                              │
   │  {type:1, to:"bob",          │                              │
   │   content:"hello"}           │                              │
   │─────────────────────────────→│                              │
   │                              │  2. ReadPump 解析消息          │
   │                              │  3. 覆盖 from = alice         │
   │                              │  4. 生成 Snowflake msgID      │
   │                              │  5. 生成服务端时间戳            │
   │                              │                              │
   │  6. 回复 ACK (已收到)         │                              │
   │←─────────────────────────────│                              │
   │  {type:3, msg_id:xxx}        │                              │
   │                              │  7. 查本地 Hub                │
   │                              │     ├─ 在线 → send channel   │
   │                              │     └─ 不在线 → Pub/Sub 广播  │
   │                              │─────────────────────────────→│
   │                              │  8. WritePump 推送消息        │
   │                              │                              │
   │                              │  9. 协程池异步：              │
   │                              │     ├─ MySQL 持久化           │
   │                              │     └─ 不在线？写 Redis 收件箱 │
   │                              │                              │
```

### 3.2 群聊消息流（写扩散）

```
发送者 Alice               服务端                    成员 B, C, D...
   │                         │                          │
   │  消息 {type:2,           │                          │
   │   to:"group123"}        │                          │
   │────────────────────────→│                          │
   │                         │  1. 生成 msgID            │
   │  ACK                    │                          │
   │←────────────────────────│                          │
   │                         │  2. 查询群成员列表         │
   │                         │     (回调 → GroupService) │
   │                         │                          │
   │                         │  3. Fan-out 写扩散：       │
   │                         │     for member in members │
   │                         │       skip sender         │
   │                         │       SendToUserOrPublish │──→ B (在线推送)
   │                         │       async persist       │──→ C (在线推送)
   │                         │       async inbox write   │──→ D (离线→收件箱)
   │                         │                          │
```

### 3.3 连接生命周期

```
客户端                          服务端
  │                               │
  │  HTTP GET /ws?token=xxx       │
  │──────────────────────────────→│  1. 提取 JWT token
  │                               │  2. ParseToken 验证
  │  101 Switching Protocols      │  3. Upgrade WebSocket
  │←──────────────────────────────│  4. NewClient(hub, userID, conn)
  │                               │  5. hub.register <- client
  │                               │  6. Redis SetOnline(userID)
  │                               │  7. go client.WritePump()
  │                               │  8. client.ReadPump() (阻塞)
  │                               │
  │  ← Ping (每54秒)              │  WritePump ticker 触发
  │  → Pong (浏览器自动)           │  PongHandler 重置 ReadDeadline
  │                               │
  │  ... 正常消息收发 ...           │
  │                               │
  │  连接断开 / 超时               │  ReadPump 退出
  │                               │  → defer: hub.unregister <- client
  │                               │  → Hub.removeClient()
  │                               │  → Redis SetOffline(userID)
  │                               │  → close(send) → WritePump 退出
  │                               │  → conn.Close()
```

---

## 四、跨节点消息路由（多实例部署）

```
┌──────────────┐                              ┌──────────────┐
│   Node A     │                              │   Node B     │
│              │                              │              │
│  Hub         │                              │  Hub         │
│  ├─ Alice    │                              │  ├─ Bob      │
│  └─ Charlie  │                              │  └─ David    │
│              │                              │              │
│  Alice → Bob │                              │              │
│  1. 查本地Hub │                              │              │
│     Bob不在   │        Redis Pub/Sub         │              │
│  2. Publish ─┼──→ im:cross_node:push ──────→│  Subscriber  │
│              │    {target:"bob",data:...}    │  3. 收到消息   │
│              │                              │  4. SendToUser │
│              │                              │     → Bob在线  │
│              │                              │     → 投递成功  │
└──────────────┘                              └──────────────┘
```

---

## 五、gRPC 拦截器链

```
                        请求方向 →
  ┌──────────┐  ┌─────────┐  ┌───────────┐  ┌────────┐  ┌────────┐  ┌─────────┐
  │ Recovery │→ │ Metrics │→ │ RateLimit │→ │ Trace  │→ │  Auth  │→ │ Handler │
  │ panic捕获│  │ RED指标  │  │ 令牌桶    │  │TraceID │  │ JWT    │  │ 业务逻辑 │
  └──────────┘  └─────────┘  └───────────┘  └────────┘  └────────┘  └─────────┘
                        ← 响应方向

  Recovery 在最外层：确保任何 panic 都不会导致进程崩溃
  Metrics 在 RateLimit 之前：被限流的请求也要统计
  Auth 白名单：Register / Login 跳过鉴权
```

---

## 六、优雅关闭流程

```
收到 SIGTERM / SIGINT
         │
         ▼
  1. ws.GlobalHub.Close()
     ├─ 取消 Pub/Sub 订阅 (pubsubCancel)
     └─ close(closeCh) → 关闭所有 Client 连接
         │
         ▼
  2. httpServer.Shutdown(timeout)
     └─ 停止接受新连接，等待存量请求完成
         │
         ▼
  3. grpcServer.GracefulStop()
     └─ 停止接受新 RPC，等待进行中的 RPC 完成
         │
         ▼
  4. pool.GlobalPool.Close(timeout)
     └─ 关闭任务队列，WaitGroup 等待所有任务完成
         │
         ▼
  5. redis.Close()
         │
         ▼
  6. mysql.Close()
         │
         ▼
  7. logger.Close() (defer)
     └─ 刷新日志缓冲区

  原则：按依赖逆序关闭（接入层 → 业务层 → 基础设施层）
  保证：不丢消息、不丢日志、不中断进行中的请求
```

---

## 七、Redis 数据结构一览

```
┌─────────────────────────────────────────────────────────────┐
│                     Redis Key 设计                           │
├──────────────────────┬───────────┬──────────────────────────┤
│  Key 模板             │  类型     │  用途                     │
├──────────────────────┼───────────┼──────────────────────────┤
│ im:user:id:{email}   │  String   │  Email → UserID 缓存     │
│ im:user:email:{uid}  │  String   │  UserID → Email 缓存     │
│ im:online:{userID}   │  String   │  在线状态 (TTL 90s)       │
│ im:inbox:{userID}    │  ZSET     │  离线收件箱 (score=msgID) │
│ im:unread:{userID}   │  Hash     │  未读计数 (field=peerID)  │
│ im:lock:{key}        │  String   │  分布式锁 (value=nonce)   │
│ im:cross_node:push   │  Pub/Sub  │  跨节点消息通道            │
└──────────────────────┴───────────┴──────────────────────────┘
```

---

## 八、数据库 ER 图

```
┌──────────────┐       ┌───────────────────┐       ┌──────────────┐
│   im_user    │       │   im_message      │       │   im_group   │
├──────────────┤       ├───────────────────┤       ├──────────────┤
│ id (PK)      │←──┐   │ id (PK,Snowflake) │   ┌──→│ id (PK)      │
│ username     │   │   │ from_user_id (FK) │───┘   │ name         │
│ email (UQ)   │   ├───│ to_user_id   (FK) │       │ owner_id     │
│ password     │   │   │ msg_type          │       │ max_member   │
│ deleted_at   │   │   │ content_type      │       │ created_at   │
│ created_at   │   │   │ content           │       └──────┬───────┘
│ updated_at   │   │   │ status            │              │
└──────┬───────┘   │   │ created_at        │              │
       │           │   └───────────────────┘              │
       │           │                                      │
       │           │   ┌───────────────────┐   ┌──────────┴─────────┐
       │           │   │ im_friend_request │   │ im_group_member    │
       │           │   ├───────────────────┤   ├────────────────────┤
       │           ├───│ from_user         │   │ id (PK,AUTO)       │
       │           ├───│ to_user           │   │ group_id (FK,UQ)   │
       │           │   │ status            │   │ user_id  (FK,UQ)   │
       │           │   │ message           │   │ role               │
       │           │   │ created_at        │   │ joined_at          │
       │           │   │ updated_at        │   └────────────────────┘
       │           │   └───────────────────┘
       │           │
       │           │   ┌───────────────────┐
       │           │   │   im_friend       │
       │           │   ├───────────────────┤
       │           ├───│ user_id   (UQ)    │    双向存储：
       │           └───│ friend_id (UQ)    │    A→B 和 B→A 各一条
       │               │ created_at        │
       │               └───────────────────┘
       │
  索引设计：
  im_message: idx_to_user_created (to_user_id, created_at)
              idx_conversation (from_user_id, to_user_id)
  im_group_member: uk_group_user (group_id, user_id) UNIQUE
                   idx_user (user_id)
  im_friend: uk_user_friend (user_id, friend_id) UNIQUE
  im_friend_request: idx_from (from_user), idx_to_status (to_user, status)
```
