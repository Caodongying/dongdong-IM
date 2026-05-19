# dongdong-IM 技术文档

> 面向 Go 语言和高并发开发初学者的技术指南。
> 目标：即使不看代码，也能从本文档中理解 IM 系统涉及的核心技术和 Go 面试高频考点。

![alt text](image.png)
---

## 目录

- [dongdong-IM 技术文档](#dongdong-im-技术文档)
  - [](#)
  - [目录](#目录)
  - [一、网络协议篇](#一网络协议篇)
    - [1.1 WebSocket：为什么 IM 不用 HTTP？](#11-websocket为什么-im-不用-http)
    - [1.2 gRPC + Protobuf：为什么不用 REST？](#12-grpc--protobuf为什么不用-rest)
    - [1.3 gRPC 拦截器链：洋葱模型](#13-grpc-拦截器链洋葱模型)
  - [二、Go 并发编程篇](#二go-并发编程篇)
    - [2.1 goroutine：比线程轻 1000 倍的并发单元](#21-goroutine比线程轻-1000-倍的并发单元)
    - [2.2 channel：goroutine 之间的通信管道](#22-channelgoroutine-之间的通信管道)
    - [2.3 select：多路复用——同时等多个 channel](#23-select多路复用同时等多个-channel)
    - [2.4 context：超时控制、取消传播与值传递](#24-context超时控制取消传播与值传递)
    - [2.5 sync 包：锁、Once、WaitGroup](#25-sync-包锁oncewaitgroup)
      - [sync.Mutex：互斥锁](#syncmutex互斥锁)
      - [sync.Map：并发安全的 map](#syncmap并发安全的-map)
      - [sync.Once：只执行一次](#synconce只执行一次)
      - [sync.WaitGroup：等待一组 goroutine 完成](#syncwaitgroup等待一组-goroutine-完成)
    - [2.6 atomic/CAS：无锁并发](#26-atomiccas无锁并发)
    - [2.7 defer + panic/recover](#27-defer--panicrecover)
  - [三、协程池篇](#三协程池篇)
    - [3.1 为什么需要协程池？](#31-为什么需要协程池)
    - [3.2 核心 Worker vs 临时 Worker](#32-核心-worker-vs-临时-worker)
    - [3.3 三级提交策略](#33-三级提交策略)
    - [3.4 优雅关闭](#34-优雅关闭)
  - [四、WebSocket 长连接篇](#四websocket-长连接篇)
    - [4.1 Hub：连接管理中心](#41-hub连接管理中心)
    - [4.2 Client：双 goroutine 模型](#42-client双-goroutine-模型)
    - [4.3 心跳保活：Ping/Pong 机制](#43-心跳保活pingpong-机制)
    - [4.4 多端登录策略](#44-多端登录策略)
  - [五、消息系统篇](#五消息系统篇)
    - [5.1 单聊消息的完整链路](#51-单聊消息的完整链路)
    - [5.2 群聊：写扩散 vs 读扩散](#52-群聊写扩散-vs-读扩散)
    - [5.3 离线消息：Redis ZSET 收件箱](#53-离线消息redis-zset-收件箱)
    - [5.4 消息可靠投递：ACK 机制](#54-消息可靠投递ack-机制)
    - [5.5 跨节点消息路由：Redis Pub/Sub](#55-跨节点消息路由redis-pubsub)
  - [六、ID 生成篇：Snowflake 算法](#六id-生成篇snowflake-算法)
    - [6.1 为什么不用自增 ID 或 UUID？](#61-为什么不用自增-id-或-uuid)
    - [6.2 Snowflake 的 64 位结构](#62-snowflake-的-64-位结构)
    - [6.3 时钟回拨问题](#63-时钟回拨问题)
  - [七、Redis 高级篇](#七redis-高级篇)
    - [7.1 分布式锁：从简单到生产级](#71-分布式锁从简单到生产级)
    - [7.2 Pub/Sub：跨节点消息总线](#72-pubsub跨节点消息总线)
    - [7.3 在线状态：String + TTL](#73-在线状态string--ttl)
    - [7.4 批量查询：MGET 与 Pipeline](#74-批量查询mget-与-pipeline)
  - [八、MySQL 与 GORM 篇](#八mysql-与-gorm-篇)
    - [8.1 连接池](#81-连接池)
    - [8.2 事务：好友双向写入](#82-事务好友双向写入)
    - [8.3 索引设计](#83-索引设计)
    - [8.4 软删除](#84-软删除)
  - [九、认证与安全篇](#九认证与安全篇)
    - [9.1 JWT：无状态认证](#91-jwt无状态认证)
    - [9.2 bcrypt：密码加密](#92-bcrypt密码加密)
    - [9.3 分片锁：高并发下的邮箱唯一性检查](#93-分片锁高并发下的邮箱唯一性检查)
  - [十、限流篇：令牌桶算法](#十限流篇令牌桶算法)
    - [10.1 四种限流算法对比](#101-四种限流算法对比)
    - [10.2 两级限流：全局 + IP](#102-两级限流全局--ip)
  - [十一、可观测性篇](#十一可观测性篇)
    - [11.1 结构化日志：Zap + TraceID](#111-结构化日志zap--traceid)
    - [11.2 Prometheus 监控：RED 方法](#112-prometheus-监控red-方法)
    - [11.3 健康检查](#113-健康检查)
  - [十二、优雅关闭篇](#十二优雅关闭篇)
    - [12.1 为什么需要优雅关闭？](#121-为什么需要优雅关闭)
    - [12.2 关闭顺序：依赖逆序](#122-关闭顺序依赖逆序)
  - [附录：面试高频问题速查](#附录面试高频问题速查)

---

## 一、网络协议篇

### 1.1 WebSocket：为什么 IM 不用 HTTP？

**HTTP 的问题：只能客户端主动请求**

HTTP 是"请求-响应"模型：客户端发请求，服务端返回响应，然后连接结束（或复用）。如果你想知道有没有新消息，只能不停地问服务器："有新消息吗？有新消息吗？"——这叫**轮询（Polling）**。

轮询的缺点很明显：
- **浪费资源**：90% 的请求得到的回答都是"没有"，白白消耗带宽和服务器 CPU
- **延迟高**：假设你每 3 秒轮询一次，消息最长要等 3 秒才能收到
- **连接开销**：每次请求都要建立/复用 TCP 连接、发送 HTTP 头部（几百字节）

**WebSocket 的解决方案：全双工长连接**

WebSocket 是一种在单个 TCP 连接上实现**全双工通信**的协议。"全双工"意味着客户端和服务端可以**同时**互相发送数据，不需要一方等另一方。

它的工作流程：
1. 客户端发送一个特殊的 HTTP 请求（带 `Upgrade: websocket` 头）
2. 服务端返回 `101 Switching Protocols`，表示同意升级
3. 从此，这个 TCP 连接就变成了 WebSocket 连接，双方可以随时互发数据
4. 连接一直保持，直到某一方主动关闭

```
普通 HTTP：
客户端 --请求--> 服务端
客户端 <--响应-- 服务端
（连接可能断开）

WebSocket：
客户端 <=====全双工通道=====> 服务端
（连接持续保持，双方随时发消息）
```

**在本项目中的使用**

客户端通过 `GET /ws?token=xxx` 发起 WebSocket 连接。服务端先验证 JWT Token（鉴权），验证通过后将 HTTP 连接升级为 WebSocket。之后，客户端发送的聊天消息和服务端推送的新消息都通过这个连接传输。

> **面试考点**：WebSocket 和 HTTP 的区别？WebSocket 的握手过程？为什么 IM 要用 WebSocket？

### 1.2 gRPC + Protobuf：为什么不用 REST？

**什么是 gRPC？**

gRPC 是 Google 开发的远程过程调用（RPC）框架。简单来说，它让你像调用本地函数一样调用远程服务器上的函数。

对比 REST API：
| 特性 | REST (JSON) | gRPC (Protobuf) |
|------|-------------|-----------------|
| 数据格式 | JSON（文本，可读） | Protobuf（二进制，紧凑） |
| 传输协议 | HTTP/1.1 | HTTP/2 |
| 序列化速度 | 慢（文本解析） | 快 3-10 倍（二进制编解码） |
| 接口约束 | 弱（文档约定） | 强（`.proto` 文件定义） |
| 代码生成 | 需手写 | 自动生成 client/server 代码 |

**什么是 Protobuf？**

Protobuf（Protocol Buffers）是一种二进制序列化格式。你在 `.proto` 文件中定义数据结构和服务接口，然后用 `protoc` 工具自动生成各种语言的代码。

```protobuf
// 定义一个 gRPC 服务
service UserService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Login(LoginRequest) returns (LoginResponse);
}

// 定义请求结构
message RegisterRequest {
  string username = 1;
  string email = 2;
  string password = 3;
}
```

运行 `protoc` 后会生成 Go 代码，包含：
- `RegisterRequest` 结构体（自动序列化/反序列化）
- `UserServiceServer` 接口（你需要实现的服务端）
- `UserServiceClient` 客户端（自动生成，可以直接调用）

**为什么本项目用 gRPC？**

1. **强类型契约**：`.proto` 文件就是接口文档，改了字段编译就报错，不会出现前后端字段名拼错的问题
2. **高性能**：Protobuf 二进制格式比 JSON 小 3-10 倍，序列化快 5-100 倍
3. **多语言支持**：一份 `.proto` 文件可以生成 Go、Java、Python 等多种语言的代码

> **面试考点**：gRPC 和 REST 的区别？Protobuf 为什么比 JSON 快？HTTP/2 有什么优势？

### 1.3 gRPC 拦截器链：洋葱模型

**什么是拦截器？**

拦截器（Interceptor）就是 gRPC 版的中间件。每个 gRPC 请求在到达真正的业务处理函数之前，会依次经过一系列拦截器。这些拦截器可以做认证、日志、限流等通用逻辑。

**洋葱模型**

拦截器的执行顺序像剥洋葱：请求从外往内穿过每层拦截器，到达核心处理函数；响应再从内往外穿回来。

```
请求 →  Recovery → Metrics → RateLimit → Trace → Auth → [业务Handler]
响应 ←  Recovery ← Metrics ← RateLimit ← Trace ← Auth ← [业务Handler]
```

本项目的拦截器链（按注册顺序）：

| 顺序 | 拦截器 | 职责 |
|------|--------|------|
| 1 | Recovery | 捕获 panic，防止进程崩溃 |
| 2 | Metrics | 记录请求数、延迟（Prometheus） |
| 3 | RateLimit | 全局限流 + IP 限流 |
| 4 | Trace | 生成/传递 TraceID |
| 5 | Auth | JWT 鉴权（白名单放行注册/登录） |

**为什么 Recovery 放最外层？**

因为 Recovery 需要捕获所有内层拦截器和业务 Handler 中的 panic。如果 Auth 拦截器 panic 了，只有在它外层的 Recovery 才能捕获。

**拦截器的函数签名**

```go
func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error)
```

- `ctx`：请求上下文，可以往里面塞数据（如 TraceID、UserID）
- `req`：请求参数
- `info`：包含方法名等元信息
- `handler`：调用它就是把请求传递给下一层

每个拦截器可以在调用 `handler(ctx, req)` 前后分别做事情——前面做"请求前处理"（如鉴权），后面做"响应后处理"（如记录耗时）。

> **面试考点**：gRPC 拦截器的执行顺序？中间件/拦截器的洋葱模型？

---

## 二、Go 并发编程篇

### 2.1 goroutine：比线程轻 1000 倍的并发单元

**什么是 goroutine？**

goroutine 是 Go 语言的轻量级"线程"。创建一个 goroutine 只需要 `go` 关键字：

```go
go func() {
    // 这段代码在一个新的 goroutine 中执行
    fmt.Println("Hello from goroutine!")
}()
```

**goroutine vs 线程**

| 特性 | 操作系统线程 | goroutine |
|------|-------------|-----------|
| 初始栈大小 | ~1MB（固定） | ~2KB（动态增长） |
| 创建开销 | 微秒级（系统调用） | 纳秒级（用户态） |
| 调度方式 | 内核调度 | Go runtime 调度（GMP 模型） |
| 数量上限 | 数千个 | 数十万甚至百万个 |
| 切换开销 | 大（内核态/用户态切换） | 小（用户态切换） |

一台 8GB 内存的机器，创建操作系统线程大约能创建 8000 个（每个 1MB 栈），但创建 goroutine 可以轻松创建百万个（每个 2KB 栈）。

**GMP 调度模型（面试高频）**

Go 的调度器使用 GMP 模型管理 goroutine：
- **G（Goroutine）**：goroutine 本身，存储了栈、状态、要执行的函数
- **M（Machine）**：操作系统线程，真正执行代码的载体
- **P（Processor）**：逻辑处理器，持有本地 goroutine 队列，P 的数量默认等于 CPU 核数

调度过程：P 从本地队列中取出 G，交给 M 执行。如果本地队列空了，会从全局队列或其他 P 的队列中"偷"任务（work stealing）。

**在本项目中的使用**

- 每个 WebSocket 连接启动 2 个 goroutine（ReadPump + WritePump）
- 协程池中维护一组 goroutine 作为 worker
- gRPC server 为每个请求分配一个 goroutine
- Redis Pub/Sub 订阅在独立 goroutine 中运行

> **面试考点**：goroutine 和线程的区别？GMP 调度模型？goroutine 的栈是怎么增长的？

### 2.2 channel：goroutine 之间的通信管道

**Go 的并发哲学**

> "Don't communicate by sharing memory; share memory by communicating."
> （不要通过共享内存来通信，而要通过通信来共享内存。）

这是 Go 并发编程的核心思想。传统语言中，多个线程通过共享变量 + 锁来通信；Go 推荐用 channel 在 goroutine 之间传递数据。

**channel 基础**

```go
// 无缓冲 channel：发送方和接收方必须同时就绪
ch := make(chan int)

// 有缓冲 channel：缓冲区满之前，发送方不会阻塞
ch := make(chan int, 100)
```

| 操作 | 无缓冲 | 有缓冲（未满） | 有缓冲（已满） |
|------|--------|-------------|-------------|
| 发送 `ch <- v` | 阻塞直到有人接收 | 立即完成 | 阻塞直到有空间 |
| 接收 `v := <-ch` | 阻塞直到有人发送 | 立即完成 | 阻塞直到有数据 |
| 关闭 `close(ch)` | 所有接收者立即得到零值 | 缓冲区数据可继续读取 | 缓冲区数据可继续读取 |

**channel 的关闭语义**

```go
close(ch) // 关闭 channel

// for range 会一直读取直到 channel 关闭
for msg := range ch {
    process(msg)
}
// channel 关闭后，for range 自动退出
```

关闭 channel 是通知所有接收者"不会再有新数据了"的方式。本项目的协程池就利用这个特性实现优雅关闭：`close(taskQueue)` 后，所有 worker 的 `for range` 循环会读完剩余任务后自动退出。

**在本项目中的使用**

- `Hub.register / unregister`：有缓冲 channel（256），用于 WebSocket 连接的注册/注销
- `Client.send`：有缓冲 channel（256），WritePump 从中读取消息写入连接
- `GoroutinePool.taskQueue`：有缓冲 channel，任务队列
- `Hub.closeCh`：无缓冲 channel，关闭信号

> **面试考点**：有缓冲和无缓冲 channel 的区别？channel 关闭后读取会怎样？向已关闭的 channel 发送数据会怎样？（panic）

### 2.3 select：多路复用——同时等多个 channel

**什么是 select？**

`select` 让你可以同时等待多个 channel 操作，哪个先就绪就执行哪个：

```go
select {
case msg := <-ch1:
    // ch1 有数据，处理
case ch2 <- data:
    // ch2 有空间，发送成功
case <-time.After(5 * time.Second):
    // 5 秒超时
default:
    // 所有 channel 都没就绪，立即执行（非阻塞模式）
}
```

**经典模式：for + select**

这是 Go 中最常见的并发模式——一个 goroutine 通过循环 + select 持续监听多个 channel：

```go
for {
    select {
    case client := <-h.register:
        h.addClient(client)      // 有新连接注册
    case client := <-h.unregister:
        h.removeClient(client)   // 有连接断开
    case <-h.closeCh:
        return                   // 收到关闭信号，退出
    }
}
```

**非阻塞发送（select + default）**

```go
select {
case client.send <- data:
    // 发送成功
default:
    // send channel 满了，丢弃消息（避免阻塞发送方）
}
```

本项目中，Hub 向 Client 推送消息时使用非阻塞发送。如果客户端消费太慢导致 send channel 满了，直接丢弃消息而不是阻塞整个 Hub——这是一种**背压控制**策略。

> **面试考点**：select 的执行规则？多个 case 同时就绪怎么办？（随机选一个）select + default 的作用？

### 2.4 context：超时控制、取消传播与值传递

**为什么需要 context？**

假设用户发起一个注册请求，服务端需要查 Redis、写 MySQL、更新缓存。如果用户在中途取消了请求（比如关闭了页面），服务端应该立即停止所有后续操作，而不是继续浪费资源。

context 就是用来解决这个问题的——它在 goroutine 之间传递"取消信号"和"截止时间"。

**三种常用的 context**

```go
// 1. 超时控制：2 秒后自动取消
ctx, cancel := context.WithTimeout(parentCtx, 2*time.Second)
defer cancel()  // 重要！即使提前完成也要调用 cancel 释放资源

// 2. 手动取消
ctx, cancel := context.WithCancel(parentCtx)
cancel()  // 主动取消，所有监听 ctx.Done() 的 goroutine 都会收到信号

// 3. 携带值（TraceID 等）
ctx = context.WithValue(parentCtx, traceIDKey{}, "abc-123")
traceID := ctx.Value(traceIDKey{}).(string)  // 读取
```

**取消传播**

context 形成一棵树：父 context 取消时，所有子 context 也会被取消。

```
请求 context（超时 10s）
├── 查 Redis context（超时 1s）
├── 写 MySQL context（超时 2s）
└── 更新缓存 context（超时 2s）
```

如果请求 context 被取消（用户断开），所有子 context 也立即取消。

**在本项目中的使用**

- 注册接口中，检查邮箱用 `WithTimeout(ctx, 1s)`，写数据库用 `WithTimeout(ctx, 2s)`
- 协程池提交任务时，先检查 `ctx.Done()` 是否已触发，避免执行已取消的任务
- TraceID 通过 `context.WithValue` 贯穿整个调用链
- 分布式锁的续期协程通过 `ctx.Done()` 感知锁释放

> **面试考点**：context 的作用？WithTimeout 和 WithCancel 的区别？为什么 defer cancel() 很重要？（防止 context 泄漏）

### 2.5 sync 包：锁、Once、WaitGroup

#### sync.Mutex：互斥锁

最基本的锁，同一时刻只允许一个 goroutine 进入临界区：

```go
var mu sync.Mutex
mu.Lock()
// 临界区：同一时刻只有一个 goroutine 能执行到这里
doSomething()
mu.Unlock()
```

本项目中，Snowflake ID 生成器用 Mutex 保护 `lastTime + sequence` 的一致性——因为这两个变量之间有依赖关系，不能用 atomic 单独操作。

#### sync.Map：并发安全的 map

Go 的内置 map **不是**并发安全的——多个 goroutine 同时读写 map 会导致程序崩溃（`fatal error: concurrent map read and map write`）。

`sync.Map` 是官方提供的并发安全 map，适合两种场景：
1. **读多写少**：key 相对固定，主要是读取
2. **无竞争写**：多个 goroutine 写不同的 key

本项目中的 Hub 用 `sync.Map` 管理 WebSocket 连接（`key=userID, value=*Client`）。不同用户的上下线操作的是不同 key，符合场景 2。如果用普通 `map + Mutex`，所有用户的操作都竞争同一把锁，高并发时成为瓶颈。

#### sync.Once：只执行一次

```go
var once sync.Once
once.Do(func() {
    // 无论调用多少次 Do，这个函数只会执行一次
    initialize()
})
```

本项目中，Client 的 `Close()` 方法用 `sync.Once` 保证只执行一次。因为 ReadPump 和 WritePump 都可能触发关闭，但 `close(channel)` 调用两次会 panic。

#### sync.WaitGroup：等待一组 goroutine 完成

```go
var wg sync.WaitGroup

for i := 0; i < 10; i++ {
    wg.Add(1)            // 计数 +1
    go func() {
        defer wg.Done()  // 计数 -1
        doWork()
    }()
}

wg.Wait()  // 阻塞直到计数归零（所有 goroutine 完成）
```

本项目的协程池用 WaitGroup 追踪所有 worker goroutine，关闭时调用 `wg.Wait()` 确保所有任务执行完毕。

> **面试考点**：sync.Map 的适用场景？sync.Once 怎么实现的？WaitGroup 的 Add/Done/Wait 分别做什么？

### 2.6 atomic/CAS：无锁并发

**什么是 CAS？**

CAS（Compare-And-Swap）是 CPU 提供的原子指令：比较当前值是否等于预期值，如果是就更新为新值，否则什么都不做。

```go
var count atomic.Int32

count.Add(1)      // 原子 +1
count.Load()      // 原子读取
count.Store(0)    // 原子写入
count.CompareAndSwap(old, new)  // CAS：如果 count==old，则设为 new
```

**atomic vs Mutex**

| 特性 | atomic | Mutex |
|------|--------|-------|
| 性能 | 纳秒级（CPU 指令） | 微秒级（可能涉及系统调用） |
| 适用场景 | 单个变量的简单操作 | 多个变量需要一致性 |
| 竞争激烈时 | 自旋消耗 CPU | 阻塞等待 |

**CAS 自旋：协程池的无锁扩容**

本项目的协程池用 CAS 实现 worker 数量的安全扩容：

```go
for {
    current := p.workerNum.Load()       // 读取当前值
    if current >= p.maxWorkers {
        break                           // 已达上限
    }
    if p.workerNum.CompareAndSwap(current, current+1) {
        go p.tempWorker()               // CAS 成功，安全地 +1 并启动 worker
        break
    }
    // CAS 失败（被其他 goroutine 抢先修改），重新循环
}
```

这段代码的关键思路是"乐观锁"：先假设没人竞争，直接尝试修改；如果发现被别人抢先了，就重试。在竞争不激烈的情况下，这比加 Mutex 高效得多。

> **面试考点**：CAS 的原理？atomic 和 Mutex 怎么选？什么是 ABA 问题？

### 2.7 defer + panic/recover

**defer：延迟执行**

`defer` 语句会在函数返回前执行，无论函数是正常返回还是 panic：

```go
func doSomething() {
    mu.Lock()
    defer mu.Unlock()  // 保证函数退出时一定解锁，避免死锁

    // 即使这里 panic 了，defer 也会执行
    riskyOperation()
}
```

多个 defer 按**后进先出（LIFO）**顺序执行。

**panic/recover：Go 的异常处理**

Go 没有 try/catch，而是用 panic/recover：
- `panic`：类似 throw，抛出一个错误，goroutine 开始"展开"（unwind），依次执行 defer
- `recover`：类似 catch，只能在 defer 函数中调用，捕获 panic 的值

```go
defer func() {
    if r := recover(); r != nil {
        log.Printf("捕获到 panic: %v", r)
        log.Printf("堆栈: %s", debug.Stack())
    }
}()
```

**在本项目中的关键应用**

1. **gRPC Recovery 拦截器**：捕获所有 handler 中的 panic，返回 Internal Error，防止一个请求的 panic 导致整个 gRPC server 崩溃
2. **协程池 runTask**：每个任务执行时都有 `defer recover()`，保证一个任务的 panic 不影响 worker 继续服务
3. **ReadPump/WritePump**：通过 defer 注册清理逻辑（注销连接、关闭 channel）

> **面试考点**：defer 的执行顺序？panic 后 defer 还会执行吗？recover 在什么位置才有效？

---

## 三、协程池篇

### 3.1 为什么需要协程池？

虽然 goroutine 很轻量，但"无限制创建"仍然有问题：

| 问题 | 说明 |
|------|------|
| 内存爆炸 | 每个 goroutine ~2KB 栈，100 万个就是 2GB |
| 调度开销 | goroutine 太多时，Go 调度器需要频繁切换，性能下降 |
| 无法感知 | `go func()` 启动后，你无法知道它是否完成（无法优雅关闭） |
| 无 panic 保护 | 一个 goroutine panic 整个进程挂掉 |

协程池解决了以上所有问题：**数量可控、优雅关闭、panic 隔离**。

### 3.2 核心 Worker vs 临时 Worker

本项目的协程池有两种 worker：

**核心 Worker**（常驻）
- 数量固定（如 4 个），启动后永远不退出
- 通过 `for range taskQueue` 循环消费任务
- 只有 `close(taskQueue)` 时才退出

**临时 Worker**（弹性伸缩）
- 任务队列满了才会创建，空闲超时后自动退出
- 通过 `select` 同时监听任务队列和超时 Timer
- 超时时间到了还没有新任务就退出

```
低流量时：4 个核心 Worker 就够用
              ┌─────────────────────────┐
任务队列 ───→ │ Core1 Core2 Core3 Core4 │
              └─────────────────────────┘

突发流量时：自动扩容临时 Worker
              ┌─────────────────────────────────────────┐
任务队列 ───→ │ Core1 Core2 Core3 Core4 Temp1 Temp2 ... │
              └─────────────────────────────────────────┘

流量恢复后：临时 Worker 空闲超时，自动退出
              ┌─────────────────────────┐
任务队列 ───→ │ Core1 Core2 Core3 Core4 │
              └─────────────────────────┘
```

这个设计思路和线程池（Java 的 `ThreadPoolExecutor`）类似：核心线程数 + 最大线程数 + 空闲超时。

### 3.3 三级提交策略

提交一个任务到协程池，会经历三个阶段：

```
提交任务
   │
   ▼
第一级：直接入队（select + default，非阻塞）
   │ 成功 → 返回 nil
   │ 失败（队列满）
   ▼
第二级：CAS 扩容临时 Worker，再尝试入队
   │ 成功 → 返回 nil
   │ 失败（Worker 已达上限或队列仍满）
   ▼
第三级：最后再尝试一次入队
   │ 成功 → 返回 nil
   │ 失败 → 返回 ErrPoolFull
```

为什么要三级？第一级是"最快路径"——大多数时候队列没满，一次 select 就搞定了。只有队列满了才进入第二级（CAS 扩容），这是"慢路径"。

### 3.4 优雅关闭

协程池的关闭流程：

1. **设置 stopped 标记**：用 `CAS(false, true)` 保证只关闭一次，之后拒绝新任务
2. **close(taskQueue)**：核心 Worker 的 `for range` 会读完剩余任务后退出；临时 Worker 的 `select` 也会感知到 channel 关闭
3. **wg.Wait()**：等待所有 Worker 退出（带超时保护）

关键点：`close(channel)` 不会丢弃缓冲区中的数据——`for range` 会先把剩余数据全部读完，然后才退出循环。这保证了已提交的任务不会丢失。

> **面试考点**：协程池的设计？核心线程和临时线程的区别？如何优雅关闭协程池？close channel 后缓冲区的数据怎么处理？

---

## 四、WebSocket 长连接篇

### 4.1 Hub：连接管理中心

Hub 是 IM 系统的核心组件，负责管理所有在线用户的 WebSocket 连接。

**核心数据结构**

```
Hub
├── clients    sync.Map    // userID → *Client 的映射
├── register   chan *Client // 注册 channel（缓冲 256）
├── unregister chan *Client // 注销 channel（缓冲 256）
├── onlineCount atomic.Int64 // 在线人数
└── closeCh    chan struct{} // 关闭信号
```

**为什么用 channel 处理注册/注销？**

将操作序列化到 channel 中，由 `Hub.Run()` 单个 goroutine 消费。这样注册/注销逻辑在单线程中执行，天然避免了并发问题。

但 `SendToUser` 是从外部 goroutine 调用的（比如处理消息的 goroutine），需要直接读 `clients` map。所以 `clients` 使用 `sync.Map`，兼顾两种访问模式。

### 4.2 Client：双 goroutine 模型

每个 WebSocket 连接由一个 `Client` 对象表示，启动两个 goroutine：

```
                    ┌─────────────────┐
客户端 ─── WS 连接 ─┤  Client         │
                    │                 │
                    │  ReadPump (G1)  │──→ 读取客户端消息 → 分发处理
                    │       ↓         │
                    │  send channel   │
                    │       ↓         │
                    │  WritePump (G2) │──→ 从 channel 取消息 → 写入连接
                    └─────────────────┘
```

**为什么需要两个 goroutine？**

WebSocket 连接**不支持并发读写**——同时调用 `ReadMessage` 和 `WriteMessage` 会导致数据竞争（data race）。Go 的标准做法是：
- ReadPump 独占读操作
- WritePump 独占写操作
- 两者通过 `send channel` 解耦

**goroutine 退出链**

```
ReadPump 读到错误（断开/超时）
   │
   ├─→ defer: hub.unregister <- client （通知 Hub 注销）
   │
   └─→ Client.Close()
         ├─→ close(send)  ← WritePump 感知到 channel 关闭，退出
         └─→ conn.Close()
```

ReadPump 退出 → 关闭 send channel → WritePump 退出。这是 Go 中用 channel 协调 goroutine 生命周期的经典模式。

### 4.3 心跳保活：Ping/Pong 机制

**为什么需要心跳？**

TCP 连接可能"假死"——客户端断网/崩溃时不会发送关闭帧，服务端以为连接还活着。心跳就是定期探活的机制。

**本项目的心跳设计**

```
WritePump 每 54 秒发送 Ping
                │
                ▼
客户端收到 Ping → 自动回复 Pong（浏览器内置行为）
                │
                ▼
ReadPump 的 PongHandler 收到 Pong → 重置读超时（+60 秒）
                │
        如果 60 秒内没收到 Pong
                │
                ▼
ReadPump 读超时 → 返回错误 → 退出 → 触发连接清理
```

**时间参数设计**

- Ping 间隔 = 54 秒
- Pong 超时 = 60 秒
- Pong 超时 > Ping 间隔，留出网络延迟的余量

**time.Ticker vs time.Timer**

- `Ticker`：周期性触发（每隔 N 秒），适合心跳。用完必须 `Stop()`，否则会泄漏
- `Timer`：一次性触发（N 秒后），适合超时。Reset 前需要 drain channel

> **面试考点**：WebSocket 如何检测死连接？Ping/Pong 的工作原理？Ticker 不 Stop 会怎样？

### 4.4 多端登录策略

IM 系统通常需要处理同一用户在多个设备登录的情况。常见策略：

| 策略 | 行为 | 适用场景 |
|------|------|---------|
| 后者踢前者 | 新连接上来，关闭旧连接 | 本项目采用 |
| 前者阻止后者 | 已有连接则拒绝新连接 | 安全性要求高 |
| 多端共存 | 允许多个连接，消息广播给所有端 | 微信（手机+电脑） |

本项目实现"后者踢前者"：在 `addClient` 时，如果 `sync.Map` 中已有该 userID 的连接，先关闭旧连接再注册新连接。

---

## 五、消息系统篇

### 5.1 单聊消息的完整链路

一条单聊消息从发出到接收，经历以下步骤：

```
用户A (发送者)                    服务端                        用户B (接收者)
    │                              │                              │
    │─── WebSocket 发送消息 ──────→│                              │
    │                              │                              │
    │                     1. 服务端覆盖 from 字段（防伪造）        │
    │                     2. Snowflake 生成消息 ID                │
    │                     3. 生成服务端时间戳                      │
    │                              │                              │
    │←── 立即回 ACK（已收到） ──────│                              │
    │                              │                              │
    │                     4. 尝试推送给 B：                        │
    │                        - 本地 Hub 找到 → 直接推送           │
    │                        - 本地没有 → Redis Pub/Sub 跨节点     │
    │                              │───── 推送消息 ──────────────→│
    │                              │                              │
    │                     5. 异步持久化（协程池）：                 │
    │                        - MySQL 写入消息记录                  │
    │                        - B 不在线 → Redis 收件箱             │
```

**关键设计决策**

1. **from 字段由服务端覆盖**：客户端可能伪造发送者身份，服务端从 WebSocket 连接的鉴权信息中取真实 userID
2. **消息 ID 由服务端生成**：Snowflake ID 保证全局有序，客户端无法伪造
3. **先回 ACK 再持久化**：降低发送延迟。持久化是异步的，失败了消息也已经推送给接收者了
4. **持久化用协程池**：不阻塞 WebSocket 读循环，提高吞吐量

### 5.2 群聊：写扩散 vs 读扩散

群消息的分发有两种模型：

**写扩散（Fan-out on Write）——本项目采用**

```
Alice 发送群消息 "Hello"（群有 Bob、Carol、Dave 三个成员）
   │
   ├─→ 写入 Bob 的收件箱
   ├─→ 写入 Carol 的收件箱
   └─→ 写入 Dave 的收件箱
```

- 优点：读取时直接从自己的收件箱拉，速度快
- 缺点：写入量 = 群成员数，大群写入开销大
- 适合：<500 人的小群

**读扩散（Fan-out on Read）**

```
Alice 发送群消息 "Hello"
   │
   └─→ 写入群消息表（只写一次）

Bob 读取消息时：查询所有他所在群的消息表
```

- 优点：写入只有一次
- 缺点：读取时需要合并多个群的消息，查询复杂
- 适合：万人群、频道

**实际应用中**，微信小群用写扩散，微信公众号/钉钉大群用读扩散或混合模型。

### 5.3 离线消息：Redis ZSET 收件箱

当接收者不在线时，消息需要存储起来等对方上线后拉取。本项目用 Redis 的 ZSET（有序集合）实现：

```
ZADD im:inbox:{userID} {msgID} {serialized_message}
```

- **key**：`im:inbox:{userID}`，每个用户一个收件箱
- **score**：消息 ID（Snowflake ID，天然有序）
- **member**：序列化后的消息内容

**为什么用 ZSET？**

1. **天然有序**：Snowflake ID 作为 score，消息自动按时间排序
2. **游标分页**：用 `ZRANGEBYSCORE` 按 score 范围查询，支持基于 lastMsgID 的游标分页
3. **去重**：ZSET 的 member 唯一，不会有重复消息

**拉取流程**

```
客户端上线 → 发送拉取请求（带 lastMsgID）
             → ZRANGEBYSCORE im:inbox:{userID} (lastMsgID, +inf LIMIT 0 50
             → 返回最多 50 条未读消息
             → 客户端确认（ACK） → ZREM 移除已确认消息
```

### 5.4 消息可靠投递：ACK 机制

IM 系统必须保证消息不丢失。本项目通过 ACK 机制实现：

```
发送者 ──消息──→ 服务端 ──→ ACK（服务端已收到）──→ 发送者
                   │
                   ├─→ 推送给接收者（在线）
                   │      ↓
                   │   接收者发送 ACK ──→ 服务端 ──→ 从收件箱移除
                   │
                   └─→ 写入收件箱（离线）
                          ↓
                       接收者上线 → 拉取 → ACK → 从收件箱移除
```

两层 ACK：
1. **发送 ACK**：服务端告诉发送者"我收到你的消息了"（立即回复）
2. **接收 ACK**：接收者告诉服务端"我收到这条消息了"（从收件箱移除）

### 5.5 跨节点消息路由：Redis Pub/Sub

**为什么需要跨节点？**

部署多个实例时，用户 A 可能在节点 1，用户 B 在节点 2。节点 1 的 Hub 找不到 B。

**解决方案**

```
节点 1                    Redis                    节点 2
   │                       │                         │
   │ 本地找不到 B           │                         │
   │──── PUBLISH ─────────→│                         │
   │                       │──── 消息推送 ──────────→│
   │                       │                    在本地 Hub 找到 B
   │                       │                    推送给 B
```

所有节点都 SUBSCRIBE 同一个 channel `im:cross_node:push`。当本地 Hub 找不到目标用户时，通过 PUBLISH 广播消息。目标用户所在节点的 subscriber goroutine 收到后，在本地 Hub 找到连接并推送。

**局限性**

Pub/Sub 消息不持久化——如果订阅者断开了，消息就丢了。但这没关系，因为离线消息已经写入了 Redis 收件箱作为兜底。

> **面试考点**：Pub/Sub 和消息队列的区别？多节点 IM 的消息路由怎么做？

---

## 六、ID 生成篇：Snowflake 算法

### 6.1 为什么不用自增 ID 或 UUID？

| 方案 | 问题 |
|------|------|
| MySQL 自增 ID | 分库分表时无法全局唯一；暴露业务量（用户能猜出有多少数据）；高并发写入有锁竞争 |
| UUID | 128 位字符串太长（36 字符），索引效率低；完全随机不递增，导致 B+ 树频繁页分裂，写入性能差 |
| Snowflake | 64 位整数，趋势递增（B+ 树友好），全局唯一，不依赖数据库，高性能 |

### 6.2 Snowflake 的 64 位结构

```
| 1 bit 符号位 | 41 bit 时间戳 | 10 bit 机器ID | 12 bit 序列号 |
|    固定为 0   |   毫秒级     | 0~1023       |  0~4095      |
```

- **符号位**：固定为 0，保证 ID 为正数
- **时间戳**：相对于自定义 epoch 的毫秒数，41 bit 可用约 69 年
- **机器 ID**：支持 1024 个节点，部署时每个节点配置不同的 ID
- **序列号**：同一毫秒内的自增序列，每毫秒最多生成 4096 个 ID

**ID 的生成过程**

```go
id = ((now - epoch) << 22) | (machineID << 12) | sequence
```

通过位运算将三部分拼接成一个 64 位整数。由于时间戳在最高位，ID 整体是趋势递增的。

**为什么趋势递增很重要？**

MySQL 的 InnoDB 引擎使用 B+ 树索引。如果主键是随机的（如 UUID），每次插入都可能在树的中间位置，导致频繁的"页分裂"（将一个数据页拆成两个）。如果主键是递增的，新数据总是追加在末尾，几乎不会页分裂，写入性能好很多。

### 6.3 时钟回拨问题

Snowflake 依赖系统时钟。如果服务器时间被调回（NTP 同步、手动修改），可能导致生成重复的 ID。

本项目的处理策略：
- **小幅回拨（≤5ms）**：等待到上次时间，容忍短暂阻塞
- **大幅回拨（>5ms）**：直接报错，拒绝生成

> **面试考点**：Snowflake 的结构？为什么 ID 是趋势递增的？时钟回拨怎么处理？和 UUID 的区别？

---

## 七、Redis 高级篇

### 7.1 分布式锁：从简单到生产级

**为什么需要分布式锁？**

单机用 `sync.Mutex` 就够了，但分布式部署时（多个进程/多台机器），内存锁无法跨进程生效。Redis 分布式锁通过一个共享的 Redis 实例来协调。

**第一版：最简单的锁**

```
加锁：SET lock_key "1" NX EX 10
      NX = 只在 key 不存在时设置（互斥）
      EX = 10 秒过期（防死锁）
解锁：DEL lock_key
```

问题：如果 A 加了锁但执行超过 10 秒，锁自动过期，B 拿到了锁。此时 A 执行完了，DEL 删除的是 B 的锁——**误删**。

**第二版：nonce 防误删**

```
加锁：SET lock_key {random_nonce} NX EX 10
解锁：if GET lock_key == my_nonce then DEL lock_key
```

每个客户端加锁时设置一个随机值（nonce），解锁时先验证 nonce 是否匹配。

问题：GET 和 DEL 不是原子操作——GET 之后、DEL 之前，锁可能过期并被别人获取。

**第三版：Lua 脚本保证原子性**

```lua
-- 解锁脚本：验证 + 删除 原子执行
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1])
else
    return 0
end
```

Redis 执行 Lua 脚本时是原子的（不会被其他命令打断），完美解决了原子性问题。

**第四版：自动续期（本项目实现）**

```
加锁成功后 → 启动一个 goroutine（watchdog）
           → 每隔 N 秒自动延长锁的过期时间
           → 直到主动解锁（cancel context 通知 watchdog 退出）
```

续期也用 Lua 脚本保证原子性：

```lua
-- 续期脚本：验证 nonce + 续期
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('EXPIRE', KEYS[1], ARGV[2])
else
    return 0  -- nonce 不匹配，说明锁已被抢占
end
```

**完整的生命周期**

```
TryLock()
   ├─ 生成 nonce
   ├─ SET key nonce NX EX ttl
   ├─ 成功 → 启动续期 goroutine → 返回 true
   └─ 失败 → 返回 false

Unlock()
   ├─ cancel context → 通知续期 goroutine 退出
   └─ Lua 脚本：验证 nonce → DEL key
```

> **面试考点**：Redis 分布式锁怎么实现？如何防止误删？Lua 脚本的作用？自动续期怎么做？RedLock 了解吗？

### 7.2 Pub/Sub：跨节点消息总线

（详见 [5.5 跨节点消息路由](#55-跨节点消息路由redis-pubsub)）

**Pub/Sub vs Kafka**

| 特性 | Redis Pub/Sub | Kafka |
|------|-------------|-------|
| 持久化 | 不持久化 | 持久化到磁盘 |
| 消费模式 | 广播（所有订阅者都收到） | 消费者组（每组只有一个消费者收到） |
| 离线恢复 | 断开后消息丢失 | 可从 offset 回溯 |
| 适用场景 | 实时推送 | 消息落库、异步处理 |

本项目用 Pub/Sub 做实时跨节点推送，离线消息的可靠性由 Redis ZSET 收件箱保证。

### 7.3 在线状态：String + TTL

```
上线：SET im:online:{userID} "1" EX 90
下线：DEL im:online:{userID}
查询：EXISTS im:online:{userID}
批量：MGET im:online:user1 im:online:user2 ...
```

TTL 设为 90 秒（心跳间隔 54 秒的约 1.5 倍）。如果服务端崩溃来不及 DEL，90 秒后 key 自动过期，不会出现永久"假在线"。

### 7.4 批量查询：MGET 与 Pipeline

**为什么需要批量操作？**

查询好友列表的在线状态时，如果有 100 个好友，循环调用 100 次 GET，每次一个 RTT（网络往返时间，通常 1-5ms），总共需要 100-500ms。

**MGET：一次查多个 key**

```
MGET im:online:user1 im:online:user2 ... im:online:user100
```

一次 RTT 完成 100 个查询，耗时 1-5ms。性能提升 100 倍。

**Pipeline：打包多个不同命令**

Pipeline 将多个命令打包发送，一次 RTT 获取所有结果。本项目的用户注册缓存更新就用 Pipeline 同时写两个 key（email→userID 和 userID→email 双向映射）。

> **面试考点**：MGET 和 Pipeline 的区别？Redis 缓存穿透/雪崩/击穿怎么解决？

---

## 八、MySQL 与 GORM 篇

### 8.1 连接池

**为什么需要连接池？**

每次操作数据库都建立新的 TCP 连接非常昂贵（TCP 三次握手 + MySQL 认证）。连接池预先创建一批连接，复用它们。

GORM 底层使用 Go 标准库 `database/sql` 的连接池：

```go
sqlDB.SetMaxOpenConns(100)    // 最大打开连接数
sqlDB.SetMaxIdleConns(10)     // 最大空闲连接数
sqlDB.SetConnMaxLifetime(1h)  // 连接最大生存时间
```

- `MaxOpenConns`：限制同时使用的连接数，防止数据库被压垮
- `MaxIdleConns`：空闲连接保持数，避免频繁创建销毁
- `ConnMaxLifetime`：连接存活上限，防止使用被数据库服务端关闭的过期连接

### 8.2 事务：好友双向写入

添加好友时需要同时写入两条记录（A→B 和 B→A），这两条记录必须**要么都成功，要么都失败**——这就是事务的 ACID 特性。

```go
db.Transaction(func(tx *gorm.DB) error {
    // 写入 A → B 的好友关系
    if err := tx.Create(&Friend{UserID: a, FriendID: b}).Error; err != nil {
        return err  // 返回 error 会自动回滚
    }
    // 写入 B → A 的好友关系
    if err := tx.Create(&Friend{UserID: b, FriendID: a}).Error; err != nil {
        return err  // 自动回滚第一条也不会写入
    }
    return nil  // 返回 nil 自动提交
})
```

GORM 的 `Transaction` 方法自动处理了 `BEGIN/COMMIT/ROLLBACK`。

> **面试考点**：事务的 ACID 特性？MySQL 的隔离级别？GORM 事务怎么用？

### 8.3 索引设计

消息表的核心查询是"查两个用户之间的聊天记录"：

```sql
SELECT * FROM im_message
WHERE from_user_id = ? AND to_user_id = ?
ORDER BY id DESC
LIMIT 50;
```

需要在 `(from_user_id, to_user_id)` 上建联合索引。联合索引的**最左前缀原则**：

```
索引 (A, B, C) 可以加速：
  WHERE A = ?
  WHERE A = ? AND B = ?
  WHERE A = ? AND B = ? AND C = ?

不能加速：
  WHERE B = ?        ← 不走索引（没有 A）
  WHERE B = ? AND C = ? ← 不走索引
```

> **面试考点**：联合索引的最左前缀原则？为什么 Snowflake ID 作为主键比 UUID 好？什么是覆盖索引？

### 8.4 软删除

GORM 的软删除不会真正删除数据，而是设置 `deleted_at` 字段：

```go
type User struct {
    ID        string
    DeletedAt gorm.DeletedAt `gorm:"index"` // 软删除字段
}

// DELETE 实际执行的是 UPDATE ... SET deleted_at = NOW()
db.Delete(&user)

// 普通查询自动过滤已删除记录
// SELECT * FROM users WHERE deleted_at IS NULL
db.Find(&users)
```

好处：数据可恢复，审计留痕。代价：需要在查询条件中加入 `deleted_at IS NULL`。

---

## 九、认证与安全篇

### 9.1 JWT：无状态认证

**什么是 JWT？**

JWT（JSON Web Token）是一种将用户身份信息编码到一个字符串中的标准。服务端生成 JWT 给客户端，客户端之后每次请求都携带 JWT，服务端验证签名即可知道"你是谁"。

**JWT 的结构**

```
eyJhbGciOiJIUzI1NiJ9.eyJ1c2VyX2lkIjoiMTIzIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
|_____ Header _____||______ Payload ______||_________ Signature __________|
```

- **Header**：算法（HS256）+ 类型（JWT）
- **Payload**：用户信息（userID、email、过期时间）
- **Signature**：用密钥对前两部分签名，防止篡改

**有状态 vs 无状态认证**

| 特性 | Session（有状态） | JWT（无状态） |
|------|-------------------|--------------|
| 存储位置 | 服务端（Redis/内存） | 客户端（Header/Cookie） |
| 扩展性 | 需要共享 session 存储 | 天然支持分布式（无需共享） |
| 注销 | 删除 session 即可 | 无法主动失效（除非黑名单） |
| 性能 | 每次需查 session 存储 | 只需验证签名（计算即可） |

本项目选择 JWT 因为 IM 系统天然是分布式的（多节点部署），JWT 不需要共享存储。

**本项目的 JWT 配置**

- 签名算法：HS256（对称加密，速度快）
- 有效期：180 分钟
- Payload 包含：userID、email、签发者、过期时间

**Token 在不同协议中的传递方式**

- gRPC：通过 metadata（类似 HTTP Header），`authorization: Bearer <token>`
- WebSocket：通过 URL 参数 `/ws?token=xxx`（因为浏览器的 WebSocket API 不支持自定义 Header）

> **面试考点**：JWT 的结构？JWT 和 Session 的区别？JWT 怎么续期？JWT 的缺点？

### 9.2 bcrypt：密码加密

**为什么不用 MD5/SHA256？**

MD5 和 SHA256 是哈希函数，设计目标是**快**。这恰恰是密码存储的噩梦——攻击者可以用 GPU 每秒计算数十亿次 MD5，暴力破解密码。

**bcrypt 的设计理念**

bcrypt 专为密码存储设计，核心特点是**故意慢**：

```go
// cost=12 意味着 2^12 = 4096 轮哈希迭代
hashPwd, _ := bcrypt.GenerateFromPassword([]byte(password), 12)
```

- **cost factor**：可调节的计算成本，每增加 1，计算时间翻倍
- **内置 salt**：每次加密自动生成随机 salt，即使相同密码也产生不同哈希
- **验证时不需要单独存 salt**：salt 嵌入在哈希值中

```
bcrypt 哈希值格式：
$2a$12$WApznUPhDubN0Mxxyz123OeXyz789abcdefghijklmnopqrstuv
|alg|cost|_______ salt ______|_________ hash ___________|
```

> **面试考点**：为什么用 bcrypt 不用 MD5？bcrypt 的 cost 参数是什么？如何防止彩虹表攻击？

### 9.3 分片锁：高并发下的邮箱唯一性检查

**问题场景**

注册时需要检查邮箱是否已存在。"检查 + 创建"必须是原子操作，否则两个请求同时通过检查，都去创建，导致邮箱重复。

**方案对比**

| 方案 | 优劣 |
|------|------|
| 全局 Mutex | 所有注册请求串行化，性能差 |
| 数据库唯一索引 | 并发时大量主键冲突错误（虽然能保证唯一性） |
| 分布式锁（Redis） | 有网络开销 |
| 分片锁 | 只锁同一个邮箱的请求，不同邮箱可并行 |

**分片锁原理**

```
32 把锁：[Lock0] [Lock1] [Lock2] ... [Lock31]

注册 alice@example.com → FNV hash → 12 → 锁 Lock12
注册 bob@example.com   → FNV hash → 7  → 锁 Lock7
注册 carol@example.com → FNV hash → 12 → 锁 Lock12（等 alice 完成）
```

不同邮箱大概率映射到不同的锁，可以并行。相同邮箱映射到同一把锁，保证串行。32 个分片将全局锁的竞争降低为 1/32。

> **面试考点**：分片锁的原理？和分段锁（ConcurrentHashMap）的关系？FNV 哈希的特点？

---

## 十、限流篇：令牌桶算法

### 10.1 四种限流算法对比

| 算法 | 原理 | 优点 | 缺点 |
|------|------|------|------|
| 固定窗口 | 每个时间窗口一个计数器 | 简单 | 窗口边界可能突发 2 倍流量 |
| 滑动窗口 | 细分为多个小窗口 | 平滑 | 实现复杂 |
| 漏桶 | 请求入桶，固定速率流出 | 输出恒定 | 无法应对合理突发 |
| **令牌桶** | 固定速率生成令牌，请求消耗令牌 | 允许突发 + 限制均速 | 略复杂 |

**令牌桶详解**

```
     令牌以固定速率（rate）放入桶中
            │
            ▼
    ┌─────────────────┐
    │  ● ● ● ● ●     │ ← 桶（容量 = burst）
    │  ● ● ●         │
    └────────┬────────┘
             │
     请求到来时消耗一个令牌
     桶空则拒绝请求
```

- `rate`：每秒放入令牌数（= 平均 QPS 上限）
- `burst`：桶容量（= 允许的最大瞬时突发量）

例：`rate=1000, burst=2000` 表示平均 QPS 上限 1000，允许瞬间消耗 2000 个令牌。

**为什么 IM 选令牌桶？**

IM 消息天然有突发性（群里突然聊嗨了），令牌桶允许短时间的流量突发，比漏桶更适合。

### 10.2 两级限流：全局 + IP

本项目实现了两级限流：

```
请求到来
   │
   ▼
全局限流检查（rate=1000, burst=2000）
   │ 通过
   ▼
IP 限流检查（rate=100, burst=200）
   │ 通过
   ▼
正常处理请求
```

- **全局限流**：保护整个服务实例不被压垮
- **IP 限流**：防止单个恶意 IP 刷接口

**IP 限流器的管理**

每个 IP 一个限流器，用 `sync.Map` 存储。为了防止大量不同 IP 导致内存泄漏，每 5 分钟清理一次超过 10 分钟未活跃的限流器。

Go 标准库扩展 `golang.org/x/time/rate` 提供了令牌桶实现，底层用 atomic 操作实现无锁高性能。

> **面试考点**：四种限流算法的区别？令牌桶和漏桶的区别？限流应该放在哪一层？

---

## 十一、可观测性篇

### 11.1 结构化日志：Zap + TraceID

**为什么用 Zap 而不是标准库 log？**

Go 标准库的 `log` 输出纯文本，无法被日志收集系统（ELK、Loki）高效解析。Zap 输出结构化 JSON：

```json
{
  "level": "info",
  "ts": "2026-03-25T10:00:00Z",
  "msg": "用户注册成功",
  "trace_id": "abc-123",
  "user_id": "user_456",
  "email": "alice@example.com"
}
```

结构化日志可以被自动索引、搜索、聚合——在生产环境中排查问题时非常关键。

**TraceID：串联一次请求的所有日志**

一个注册请求可能产生 10 条日志（参数校验、查 Redis、写 MySQL、更新缓存...）。如果没有 TraceID，这些日志散落在海量日志中，很难关联。

TraceID 的传递链：
```
gRPC 请求进入 → Trace 拦截器生成 TraceID → 注入 context
    → Service 层从 context 读取 TraceID → 写入每条日志
    → 异步任务也从 context 读取 TraceID → 日志可追溯
```

> **面试考点**：结构化日志和普通日志的区别？TraceID 怎么传递？Zap 的性能为什么好？

### 11.2 Prometheus 监控：RED 方法

**可观测性三大支柱**

| 支柱 | 工具 | 用途 |
|------|------|------|
| Metrics（指标） | Prometheus + Grafana | 监控告警（CPU 使用率、QPS、延迟） |
| Logging（日志） | ELK / Loki | 排查具体问题 |
| Tracing（链路追踪） | Jaeger / Zipkin | 分析请求的完整调用链 |

**RED 方法**

RED 是监控微服务最核心的三个指标：
- **R**ate（速率）：每秒请求数
- **E**rror（错误）：每秒错误数
- **D**uration（延迟）：请求耗时分布

本项目在 gRPC 指标拦截器中自动采集 RED 指标。

**Prometheus 的四种指标类型**

| 类型 | 特点 | 本项目使用 |
|------|------|----------|
| Counter | 只增不减 | 请求总数、消息总数、限流拒绝数 |
| Gauge | 可增可减 | WebSocket 在线数、协程池活跃数 |
| Histogram | 值的分布 | gRPC 请求延迟（P50/P99） |
| Summary | 类似 Histogram | 本项目未使用 |

**Histogram 的 bucket 设计**

```go
Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5}
```

这些 bucket 定义了延迟区间（秒）。Prometheus 会统计落在每个区间的请求数量，从而计算 P50、P95、P99 等分位数。

### 11.3 健康检查

```
GET /health → {"status": "ok"}
```

K8s 用这个端点做探活：
- **livenessProbe**：检测进程是否还活着，失败就重启 Pod
- **readinessProbe**：检测是否准备好接收流量，失败就从 Service 摘除

> **面试考点**：Prometheus 的指标类型？什么是 RED 方法？Histogram 和 Summary 的区别？

---

## 十二、优雅关闭篇

### 12.1 为什么需要优雅关闭？

如果直接 `kill -9` 一个 IM 进程：
- 正在处理的 gRPC 请求可能返回错误
- WebSocket 连接被粗暴断开，客户端不知道发生了什么
- 协程池中正在执行的任务被中断，消息可能丢失
- MySQL/Redis 连接没有正确关闭，可能导致连接泄漏

优雅关闭的目标：**不丢数据、不断请求、有序清理**。

### 12.2 关闭顺序：依赖逆序

初始化是从底层到上层（先 MySQL，后 gRPC server），关闭是反过来——**先关接入层，再关业务层，最后关基础设施**。

```
收到 SIGTERM/SIGINT 信号
   │
   ▼
1. 关闭 WebSocket Hub
   ├─ 关闭 Pub/Sub 订阅
   ├─ 关闭所有 WebSocket 连接
   └─ 客户端感知断开，可以尝试重连到其他节点
   │
   ▼
2. 关闭 HTTP Server
   ├─ 停止接受新连接
   └─ 等待已有请求完成（带超时）
   │
   ▼
3. 关闭 gRPC Server（GracefulStop）
   ├─ 停止接受新 RPC
   └─ 等待进行中的 RPC 完成
   │
   ▼
4. 关闭协程池
   ├─ 拒绝新任务
   ├─ close(taskQueue)，worker drain 剩余任务
   └─ wg.Wait() 等待所有 worker 退出（带超时）
   │
   ▼
5. 关闭 Redis 连接
   │
   ▼
6. 关闭 MySQL 连接
   │
   ▼
7. 日志 flush（确保最后几条日志写入文件）
```

**信号监听**

```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
sig := <-quit  // 阻塞等待信号
```

- `SIGTERM`：K8s/Docker 优雅停止时发送
- `SIGINT`：Ctrl+C 时发送

**为什么缓冲大小是 1？**

`make(chan os.Signal, 1)` 的缓冲大小为 1，保证信号不会因为 channel 满而丢失。如果用无缓冲 channel，当主 goroutine 还没执行到 `<-quit` 时信号到来，信号会被丢弃。

> **面试考点**：优雅关闭的顺序？为什么要按依赖逆序关闭？GracefulStop 和 Stop 的区别？如何用 Go 监听系统信号？

---

## 附录：面试高频问题速查

| 话题 | 常见问题 | 本文对应章节 |
|------|---------|------------|
| 网络协议 | WebSocket 和 HTTP 的区别 | 1.1 |
| 网络协议 | gRPC 和 REST 的区别 | 1.2 |
| Go 并发 | goroutine 和线程的区别 | 2.1 |
| Go 并发 | channel 的底层实现 | 2.2 |
| Go 并发 | select 多个 case 同时就绪怎么办 | 2.3 |
| Go 并发 | context 的作用和类型 | 2.4 |
| Go 并发 | sync.Map 的适用场景 | 2.5 |
| Go 并发 | atomic 和 Mutex 怎么选 | 2.6 |
| Go 并发 | defer 的执行顺序 | 2.7 |
| 设计模式 | 协程池的设计 | 三 |
| IM 架构 | 消息可靠投递怎么保证 | 5.1, 5.4 |
| IM 架构 | 写扩散和读扩散 | 5.2 |
| IM 架构 | 多节点消息路由 | 5.5 |
| 分布式 | Snowflake ID 的结构 | 六 |
| Redis | 分布式锁怎么实现 | 7.1 |
| Redis | 缓存穿透/雪崩/击穿 | 7.4 |
| MySQL | 事务的 ACID 特性 | 8.2 |
| MySQL | 联合索引最左前缀 | 8.3 |
| 安全 | JWT 的结构和原理 | 9.1 |
| 安全 | bcrypt 和 MD5 的区别 | 9.2 |
| 系统设计 | 限流算法对比 | 十 |
| 系统设计 | 优雅关闭的顺序 | 十二 |
| 可观测性 | Prometheus 指标类型 | 11.2 |
