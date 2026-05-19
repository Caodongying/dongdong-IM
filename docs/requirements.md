# dongdong-IM 需求文档（PRD）

## 1. 项目概述

### 1.1 项目定位
dongdong-IM 是一款基于 Go 语言开发的即时通讯（IM）后端系统，支持单聊、群聊、好友管理等核心功能。项目以 gRPC 作为主要服务协议，WebSocket 作为实时消息推送通道，具备高并发、可扩展、分布式部署的能力。

### 1.2 目标用户
- 移动端 / Web 端 / 桌面端 IM 客户端
- 通过 gRPC 接口集成的第三方系统

### 1.3 技术选型
| 组件 | 技术 | 说明 |
|------|------|------|
| 语言 | Go 1.21+ | 高并发原生支持 |
| RPC 框架 | gRPC + Protobuf | 强类型契约、高性能序列化 |
| 长连接 | gorilla/websocket | 实时消息推送 |
| 数据库 | MySQL + GORM | 消息持久化、用户数据 |
| 缓存 | Redis | 在线状态、离线收件箱、分布式锁、缓存 |
| 日志 | Zap | 结构化日志 + TraceID |
| 配置 | Viper | YAML 配置 + 热更新 |
| 监控 | Prometheus | RED 指标采集 |
| ID 生成 | Snowflake | 全局有序、分布式唯一 |

---

## 2. 功能需求

### 2.1 用户模块

#### 2.1.1 用户注册
- **入口**：gRPC `UserService.Register`
- **流程**：
  1. 参数校验（用户名、邮箱格式、密码强度）
  2. 分片锁 + Redis 缓存检查邮箱唯一性
  3. bcrypt 加密密码（cost=12）
  4. Snowflake 生成用户 ID
  5. MySQL 写入用户记录
  6. 协程池异步更新 Redis 缓存（Email ↔ UserID 双向映射）
- **返回**：用户 ID、用户名
- **约束**：邮箱全局唯一

#### 2.1.2 用户登录
- **入口**：gRPC `UserService.Login`
- **流程**：
  1. 通过邮箱查询用户（优先 Redis 缓存，未命中查 MySQL）
  2. bcrypt 验证密码
  3. 生成 JWT token（HS256，有效期 180 分钟）
- **返回**：JWT token、用户 ID、用户名
- **约束**：登录接口无需鉴权（Auth 拦截器白名单）

---

### 2.2 好友模块

#### 2.2.1 发送好友申请
- **入口**：gRPC `FriendService.AddFriend`
- **流程**：
  1. 校验目标用户存在
  2. 校验不能加自己
  3. 校验是否已是好友
  4. 创建好友申请记录（status=pending）
- **返回**：申请 ID

#### 2.2.2 处理好友申请
- **入口**：gRPC `FriendService.HandleFriend`
- **流程**：
  1. 查询申请记录，校验接收者身份
  2. 接受：事务内写入双向好友关系（A→B 和 B→A），更新申请状态
  3. 拒绝：更新申请状态为 rejected
- **约束**：双向好友关系通过 MySQL 事务保证一致性

#### 2.2.3 好友列表
- **入口**：gRPC `FriendService.ListFriends`
- **流程**：
  1. 查询当前用户的好友关系
  2. 关联查询好友用户信息
  3. Redis MGET 批量查询在线状态
- **返回**：好友列表（ID、用户名、邮箱、是否在线）

#### 2.2.4 删除好友
- **入口**：gRPC `FriendService.DeleteFriend`
- **流程**：事务内删除双向好友记录
- **约束**：双向删除

#### 2.2.5 待处理申请列表
- **入口**：gRPC `FriendService.PendingRequests`
- **返回**：所有 status=pending 的好友申请

---

### 2.3 消息模块

#### 2.3.1 单聊消息
- **入口**：WebSocket 消息（type=1）
- **流程**：
  1. ReadPump 解析 JSON 消息
  2. 服务端覆盖 from 字段（防伪造）
  3. Snowflake 生成全局有序消息 ID
  4. 生成服务端时间戳
  5. 立即回复 ACK 给发送者
  6. 尝试实时推送给接收者：
     - 本地 Hub 查找 → 在线则写入 send channel
     - 本地不在 → Redis Pub/Sub 广播给其他节点
  7. 协程池异步持久化：
     - MySQL 写入消息记录
     - 接收者不在线 → 写入 Redis ZSET 收件箱
- **消息格式**：
  ```json
  {
    "type": 1,
    "msg_id": 123456789,
    "from": "alice_id",
    "to": "bob_id",
    "content_type": 1,
    "content": "hello",
    "timestamp": 1711324800000
  }
  ```

#### 2.3.2 群聊消息
- **入口**：WebSocket 消息（type=2）
- **分发模型**：写扩散（Fan-out on Write）
- **流程**：
  1. 生成消息 ID + 时间戳
  2. 回复 ACK 给发送者
  3. 查询群成员列表
  4. 遍历成员（跳过发送者），逐个推送：
     - 在线 → 实时推送（本地或跨节点）
     - 不在线 → 写入该成员的 Redis 收件箱
  5. 协程池异步持久化（每个成员一条消息记录）
- **约束**：适合 <500 人的小群，大群需要切换为读扩散

#### 2.3.3 离线消息拉取
- **入口**：gRPC `MessageService.PullOffline`
- **流程**：
  1. 以 lastMsgID 为游标，从 Redis ZSET 中 ZRANGEBYSCORE 分页拉取
  2. 反序列化消息内容
  3. 返回消息列表 + 是否还有更多
- **分页**：基于 Snowflake msgID 的游标分页（天然有序）

#### 2.3.4 消息确认（ACK）
- **入口**：gRPC `MessageService.Ack`
- **流程**：
  1. 接收消息 ID 列表
  2. 批量更新 MySQL 中消息状态为「已送达」
  3. 从 Redis 收件箱中移除已确认的消息
- **约束**：支持批量 ACK

#### 2.3.5 历史消息查询
- **入口**：gRPC `MessageService.History`
- **流程**：
  1. 根据会话双方（from + to）查询 MySQL
  2. 支持分页（offset + limit）
- **返回**：消息列表

#### 2.3.6 消息类型
| type | 含义 | 说明 |
|------|------|------|
| 1 | 单聊消息 | 点对点 |
| 2 | 群聊消息 | to 为 groupID |
| 3 | ACK | 服务端已收到确认 |
| 4 | 已读回执 | 预留 |
| 5 | 系统通知 | 预留 |
| 6 | 心跳 | 预留 |

#### 2.3.7 内容类型
| content_type | 含义 |
|-------------|------|
| 1 | 文本 |
| 2 | 图片 |
| 3 | 文件 |

---

### 2.4 群组模块

#### 2.4.1 创建群组
- **入口**：gRPC `GroupService.CreateGroup`
- **流程**：
  1. Snowflake 生成群 ID
  2. 事务内：
     - 创建群记录（名称、群主、上限）
     - 批量插入群成员（群主 role=2 + 初始成员 role=0）
- **约束**：群主自动加入，默认上限 200 人

#### 2.4.2 加入群组
- **入口**：gRPC `GroupService.JoinGroup`
- **校验**：群存在、未加入、未满员
- **流程**：插入群成员记录（role=0）

#### 2.4.3 退出群组
- **入口**：gRPC `GroupService.LeaveGroup`
- **约束**：群主不能退出（需先解散或转让）

#### 2.4.4 解散群组
- **入口**：gRPC `GroupService.DismissGroup`
- **约束**：仅群主可操作
- **流程**：事务内删除所有群成员 + 群记录

#### 2.4.5 群成员列表
- **入口**：gRPC `GroupService.ListMembers`
- **返回**：成员 ID、用户名、角色（群主/管理员/成员）、是否在线
- **在线状态**：Redis MGET 批量查询

#### 2.4.6 我的群列表
- **入口**：gRPC `GroupService.ListMyGroups`
- **返回**：群 ID、群名、群主 ID、当前成员数

---

### 2.5 长连接模块

#### 2.5.1 WebSocket 连接
- **入口**：HTTP GET `/ws?token=<jwt_token>`
- **鉴权**：从 query param 或 Authorization header 提取 JWT token
- **升级**：HTTP → WebSocket（gorilla/websocket）
- **并发模型**：每连接两个 goroutine（ReadPump + WritePump）

#### 2.5.2 心跳保活
- **机制**：Ping/Pong
  - 服务端每 54 秒发送 Ping
  - 客户端收到 Ping 后自动回复 Pong（浏览器内置）
  - 服务端 60 秒内未收到 Pong 则判定连接死亡
- **超时处理**：ReadDeadline 超时 → ReadPump 退出 → 触发连接清理

#### 2.5.3 多端登录策略
- **策略**：后者踢前者
- **行为**：同一用户新连接上来，关闭旧连接

#### 2.5.4 在线状态
- **存储**：Redis String（TTL 90 秒）
- **上线**：WebSocket 连接成功 → `SetOnline(userID)`
- **下线**：连接断开 → `SetOffline(userID)`
- **查询**：单个 `IsOnline()` / 批量 `BatchGetOnline()`

---

### 2.6 跨节点消息路由

#### 2.6.1 场景
多实例部署时，发送者和接收者可能在不同节点。

#### 2.6.2 方案
- **Redis Pub/Sub**：所有节点订阅同一频道 `im:cross_node:push`
- **流程**：
  1. 本地 Hub 查找接收者 → 不在本地
  2. 通过 Redis Pub/Sub 广播消息
  3. 接收者所在节点的 Subscriber 收到后投递给本地连接

---

## 3. 非功能需求

### 3.1 性能
| 指标 | 目标 |
|------|------|
| 单节点 WebSocket 连接数 | 10,000+ |
| 消息推送延迟（本地） | < 5ms |
| gRPC 接口 P99 延迟 | < 100ms |
| 全局限流 | 1000 QPS（可配置） |
| 单 IP 限流 | 100 QPS |

### 3.2 可靠性
- **消息不丢失**：ACK 确认 + 离线收件箱兜底
- **优雅关闭**：信号监听 → 按依赖逆序关闭，不中断进行中请求
- **Panic 恢复**：gRPC Recovery 拦截器捕获所有 panic
- **分布式锁安全**：Lua 脚本原子操作 + nonce 防误删 + 自动续期

### 3.3 可观测性
- **日志**：Zap 结构化日志 + TraceID 贯穿全链路 + 文件切割
- **指标**：Prometheus 暴露 `/metrics`
  - WebSocket 在线数（Gauge）
  - 消息计数（Counter，按类型分）
  - gRPC 请求数（Counter，按方法+状态码分）
  - gRPC 延迟分布（Histogram）
  - 协程池活跃数（Gauge）
  - 限流拒绝数（Counter）
- **健康检查**：`/health` 端点（K8s liveness/readiness probe）

### 3.4 安全性
- **鉴权**：JWT token（HS256，180 分钟有效期）
- **密码存储**：bcrypt（cost=12）
- **防伪造**：WebSocket 消息的 from 字段由服务端覆盖
- **消息大小限制**：WebSocket 最大 4096 字节
- **限流**：令牌桶算法（全局 + per-IP）

### 3.5 可扩展性
- **水平扩展**：多节点部署，通过 Redis Pub/Sub 路由跨节点消息
- **Snowflake ID**：支持 1024 台机器（10 bit 机器号）
- **协程池**：Core + Temp worker 弹性伸缩
- **配置热更新**：Viper 文件监听，运行时自动生效

---

## 4. 接口契约

### 4.1 gRPC 服务列表

| 服务 | 方法 | 是否需要鉴权 |
|------|------|-------------|
| UserService | Register | 否 |
| UserService | Login | 否 |
| FriendService | AddFriend | 是 |
| FriendService | HandleFriend | 是 |
| FriendService | ListFriends | 是 |
| FriendService | DeleteFriend | 是 |
| FriendService | PendingRequests | 是 |
| MessageService | SendMessage | 是 |
| MessageService | PullOffline | 是 |
| MessageService | Ack | 是 |
| MessageService | History | 是 |
| GroupService | CreateGroup | 是 |
| GroupService | JoinGroup | 是 |
| GroupService | LeaveGroup | 是 |
| GroupService | DismissGroup | 是 |
| GroupService | ListMembers | 是 |
| GroupService | ListMyGroups | 是 |

### 4.2 HTTP 端点

| 路径 | 方法 | 说明 |
|------|------|------|
| /ws | GET | WebSocket 升级（query: token） |
| /metrics | GET | Prometheus 指标暴露 |
| /health | GET | 健康检查 |

### 4.3 WebSocket 协议

- **传输格式**：JSON over TextMessage
- **方向**：全双工
- **心跳**：服务端 Ping → 客户端 Pong
- **消息流**：客户端发送 → 服务端 ACK → 服务端推送给接收者
