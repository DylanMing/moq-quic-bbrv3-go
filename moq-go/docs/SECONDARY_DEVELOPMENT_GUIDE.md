# MOQ-GO 二次开发文档 - 统计日志功能集成指南

## 文档信息

| 项目 | 内容 |
|------|------|
| 项目名称 | MOQ-GO 统计日志功能二次开发指南 |
| 文档版本 | v1.0 |
| 创建日期 | 2026-03-27 |
| 适用对象 | moq-go 二次开发人员 |

---

## 1. 概述

本文档详细说明如何在 moq-go 框架上进行二次开发时，集成和使用统计日志功能。通过本文档，开发人员可以：

- 理解统计日志模块的架构设计
- 掌握统计日志接口的使用方法
- 集成 QUIC 连接统计和拥塞控制统计
- 自定义日志输出格式

---

## 2. 模块架构

### 2.1 核心组件

```
moqt/stats.go
├── 接口定义
│   ├── ConnectionStatsProvider    # 连接统计提供者接口
│   ├── CongestionStatsProvider     # 拥塞统计提供者接口
│   └── ConnectionStats             # 连接统计数据结构
│
├── 数据结构
│   ├── ConnectionStats            # QUIC 连接统计
│   ├── CongestionStats             # 拥塞控制统计
│   └── BBRv3Stats                 # BBR 详细统计
│
└── 日志器
    ├── StatsLogger                 # 连接统计日志器
    └── CongestionStatsLogger       # 拥塞统计日志器
```

### 2.2 文件位置

| 文件 | 路径 | 说明 |
|------|------|------|
| 统计模块 | `moqt/stats.go` | 核心统计功能实现 |
| 会话管理 | `moqt/moqtsession.go` | MOQTSession 及 GetConnectionStats |
| 拨号器 | `moqt/moqtdialer.go` | MOQTDialer 默认配置 |
| 发布者 API | `moqt/api/pub.go` | MOQPub 统计接口 |
| 订阅者 API | `moqt/api/sub.go` | MOQSub 统计接口 |
| 拥塞接口 | `quic-go/interface.go` | 拥塞控制接口定义 |
| BBRv3 | `quic-go-bbr/internal/congestion/bbrv3.go` | BBRv3 实现 |
| Cubic | `quic-go-bbr/internal/congestion/cubic_sender.go` | Cubic 实现 |

---

## 3. 接口说明

### 3.1 连接统计接口

#### ConnectionStatsProvider 接口

```go
type ConnectionStatsProvider interface {
    GetConnectionStats() ConnectionStats
}
```

**说明**：需要实现此接口以提供 QUIC 连接统计。

#### ConnectionStats 数据结构

```go
type ConnectionStats struct {
    MinRTT        time.Duration  // 最小 RTT
    LatestRTT     time.Duration  // 最新 RTT
    SmoothedRTT   time.Duration  // 平滑 RTT
    PacketsSent   uint64         // 已发送数据包数
    PacketsLost   uint64         // 丢失数据包数
}
```

### 3.2 拥塞统计接口

#### CongestionStatsProvider 接口

```go
type CongestionStatsProvider interface {
    GetCongestionStats() CongestionStats
}
```

#### CongestionStats 数据结构

```go
type CongestionStats struct {
    CongestionWindow uint64    // 拥塞窗口（字节）
    PacingRate      uint64    // 发送速率（字节/秒）
    BytesInFlight   uint64    // 飞行中的字节数
    TotalBytesSent  uint64    // 总发送字节数
    TotalBytesLost  uint64    // 总丢失字节数
    MaxBandwidth    uint64    // 最大带宽估计
    State           string    // 状态：SlowStart/Recovery/CongestionAvoidance/Startup/Drain/Probe_BW/Probe_RTT
    InRecovery      bool      // 是否处于恢复状态
    InSlowStart     bool      // 是否处于慢启动状态
    PacingGain      float64   // Pacing 增益
    CwndGain        float64   // 拥塞窗口增益
}
```

### 3.3 拥塞控制接口（BBR/Cubic）

#### SendAlgorithmWithDebugInfos 接口

```go
type SendAlgorithmWithDebugInfos interface {
    SendAlgorithm
    InSlowStart() bool
    InRecovery() bool
    GetCongestionWindow() ByteCount
    GetStats(bytesInFlight ByteCount) BBRv3Stats
}
```

#### BBRv3Stats 数据结构

```go
type BBRv3Stats struct {
    CongestionWindow uint64
    PacingRate      uint64
    BytesInFlight   uint64
    TotalBytesSent  uint64
    TotalBytesLost  uint64
    MinRTT          time.Duration
    MaxRTT          time.Duration
    LastRTT         time.Duration
    SmoothedRTT     time.Duration
    PacingGain      float64
    CwndGain        float64
    State           string
    InRecovery      bool
    InSlowStart     bool
    MaxBandwidth    uint64
}
```

---

## 4. 快速开始

### 4.1 启用统计日志（使用 newpub/newsub）

最简单的方式是使用 newpub 和 newsub 示例程序：

```bash
# 启动订阅者（启用统计）
go run examples/newsub/newsub.go -stats -congestion=bbr3

# 启动发布者（启用统计）
go run examples/newpub/newpub.go -stats -congestion=bbr3
```

**参数说明**：
- `-stats`：启用统计日志输出
- `-congestion`：选择拥塞控制算法（cubic/bbr1/bbr3）

### 4.2 观察日志输出

启用 `-stats` 后，您将看到两类日志：

**QUIC 连接统计** `[QUIC Stats]`：
```
[QUIC Stats] MinRTT=1.234ms LatestRTT=2.345ms SmoothedRTT=2.100ms PacketsSent=100 PacketsLost=0
```

**拥塞控制统计** `[Congestion Stats]`：
```
[Congestion Stats] CWND=131.07 KB PacingRate=524.29 KB/s BytesInFlight=40.95 KB TotalBytesSent=1.23 MB State=Probe_BW ...
```

---

## 5. 二次开发集成指南

### 5.1 在自定义应用中集成统计日志

#### 步骤 1：实现 ConnectionStatsProvider 接口

```go
import "github.com/DineshAdhi/moq-go/moqt"

type MyApp struct {
    session *moqt.MOQTSession
}

func (m *MyApp) GetConnectionStats() moqt.ConnectionStats {
    if m.session != nil {
        return m.session.GetConnectionStats()
    }
    return moqt.ConnectionStats{}
}
```

#### 步骤 2：创建并启动 StatsLogger

```go
import (
    "time"
    "github.com/DineshAdhi/moq-go/moqt"
)

func main() {
    // 创建统计日志器（每 1 秒输出一次）
    statsLogger := moqt.NewStatsLogger(myApp, 1*time.Second)

    // 启动日志输出
    statsLogger.Start()

    // 在适当的时候停止
    defer statsLogger.Stop()

    // ... 您的业务逻辑 ...
}
```

### 5.2 自定义日志输出格式

#### 使用自定义回调函数

```go
import "github.com/rs/zerolog/log"

customLogFunc := func(stats moqt.ConnectionStats) {
    log.Info().
        Str("min_rtt", stats.MinRTT.String()).
        Str("s_rtt", stats.SmoothedRTT.String()).
        Uint64("sent", stats.PacketsSent).
        Uint64("lost", stats.PacketsLost).
        Msg("My Custom Stats Format")
}

statsLogger := moqt.NewStatsLoggerWithCallback(myApp, 1*time.Second, customLogFunc)
statsLogger.Start()
```

#### 输出格式示例

```json
{"level":"info","min_rtt":"1.234ms","s_rtt":"2.100ms","sent":100,"lost":0,"time":"2026-03-27T10:00:00Z","message":"My Custom Stats Format"}
```

### 5.3 选择拥塞控制算法

在创建 DialerOptions 时，通过 `Congestion` 字段指定：

```go
import "github.com/quic-go/quic-go"

options := moqt.DialerOptions{
    ALPNs: []string{"moq-00"},
    QuicConfig: &quic.Config{
        EnableDatagrams: true,
        MaxIdleTimeout: 60 * time.Second,
        Congestion: func() quic.SendAlgorithmWithDebugInfos {
            // 选择 BBRv3
            return quic.NewBBRv3(nil)

            // 或选择 BBRv1
            // return quic.NewBBRv1(nil)

            // 或选择 Cubic
            // return quic.NewCubic(nil)
        },
    },
}
```

### 5.4 在 MOQPub/MOQSub 中使用

MOQPub 和 MOQSub 已经实现了 `ConnectionStatsProvider` 接口：

```go
pub := api.NewMOQPub(options, "127.0.0.1:4443")
handler, err := pub.Connect()

// pub 已经实现了 GetConnectionStats() 方法
statsLogger := moqt.NewStatsLogger(pub, 1*time.Second)
statsLogger.Start()
```

---

## 6. 工具函数

### 6.1 字节格式化

```go
// 将字节数格式化为人类可读字符串
str := moqt.FormatBytes(131072)      // "128.00 KB"
str := moqt.FormatBytes(1048576)     // "1.00 MB"
str := moqt.FormatBytes(1073741824) // "1.00 GB"
```

### 6.2 带宽格式化

```go
// 将比特率格式化为人类可读字符串（字节/秒）
str := moqt.FormatBandwidth(1024000)      // "128.00 KB/s"
str := moqt.FormatBandwidth(1048576)      // "128.00 KB/s"
str := moqt.FormatBandwidth(1048576000)   // "128.00 MB/s"
```

**注意**：`FormatBandwidth` 会将输入值除以 8 转换为字节。

---

## 7. 错误处理

### 7.1 常见错误及解决方案

| 错误 | 原因 | 解决方案 |
|------|------|----------|
| `GetConnectionStats() returns empty stats` | session 未建立或为 nil | 确保 MOQTSession 已正确建立 |
| `statsLogger.Start()` 无输出 | provider 为 nil | 确保传入有效的 ConnectionStatsProvider |
| 连接建立后立即断开 | 拥塞控制配置错误 | 检查 `quic.NewBBRv3(nil)` 等函数调用 |

### 7.2 nil 检查最佳实践

```go
func (m *MyApp) GetConnectionStats() moqt.ConnectionStats {
    if m.session == nil {
        return moqt.ConnectionStats{}
    }
    if m.session.QuicConn == nil {
        return moqt.ConnectionStats{}
    }
    return m.session.GetConnectionStats()
}
```

---

## 8. 性能注意事项

### 8.1 日志输出频率

- 默认日志间隔：1 秒
- 建议最小间隔：500ms
- 高频日志可能影响性能

### 8.2 内存使用

`StatsLogger` 使用 channel 实现停止信号，内存占用极小。

### 8.3 Goroutine 资源

每个 `StatsLogger` 启动一个 goroutine，定期输出日志。

---

## 9. 示例代码

### 9.1 完整示例：自定义 MOQ 应用

```go
package main

import (
    "fmt"
    "time"

    "github.com/DineshAdhi/moq-go/moqt"
    "github.com/DineshAdhi/moq-go/moqt/api"
    "github.com/quic-go/quic-go"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

const STATS_INTERVAL = 1 * time.Second

func main() {
    zerolog.SetGlobalLevel(zerolog.InfoLevel)

    options := moqt.DialerOptions{
        ALPNs: []string{"moq-00"},
        QuicConfig: &quic.Config{
            EnableDatagrams: true,
            MaxIdleTimeout:  60 * time.Second,
            Congestion: func() quic.SendAlgorithmWithDebugInfos {
                return quic.NewBBRv3(nil)  // 使用 BBRv3
            },
        },
    }

    // 创建发布者
    pub := api.NewMOQPub(options, "127.0.0.1:4443")
    handler, err := pub.Connect()
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to connect")
    }

    // 启用统计日志
    statsLogger := moqt.NewStatsLogger(pub, STATS_INTERVAL)
    statsLogger.Start()

    // 设置订阅处理
    pub.OnSubscribe(func(ps moqt.PubStream) {
        log.Info().Str("track", ps.TrackName).Msg("New subscription")
    })

    // 发送 Announce
    handler.SendAnnounce("test")

    // 运行 30 秒后停止
    time.Sleep(30 * time.Second)
    statsLogger.Stop()

    fmt.Println("统计日志功能测试完成")
}
```

### 9.2 拥塞控制算法对比示例

```go
// BBRv3 配置
bbr3Config := func() quic.SendAlgorithmWithDebugInfos {
    return quic.NewBBRv3(nil)
}

// BBRv1 配置
bbr1Config := func() quic.SendAlgorithmWithDebugInfos {
    return quic.NewBBRv1(nil)
}

// Cubic 配置
cubicConfig := func() quic.SendAlgorithmWithDebugInfos {
    return quic.NewCubic(nil)
}

// 在 QuicConfig 中使用
&quic.Config{
    Congestion: bbr3Config,  // 选择其中一个
}
```

---

## 10. 相关文档

| 文档 | 说明 |
|------|------|
| [TEST_GUIDE.md](./TEST_GUIDE.md) | 测试指导文档 |
| [BBR_STATS_SUMMARY.md](./BBR_STATS_SUMMARY.md) | BBR 统计摘要 |

---

## 11. 修改历史

| 日期 | 版本 | 修改内容 | 负责人 |
|------|------|----------|--------|
| 2026-03-27 | v1.0 | 初始版本 | - |