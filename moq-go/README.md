# MOQT - GO

Simple Implementation of Media Over QUIC Transport (MOQT) in Go, in compliant with the [DRAFT04](https://dataObjectStreamer.ietf.org/doc/draft-ietf-moq-transport/04/)

This MOQT library currently supports WebTransport and QUIC Protocols.

| Module    | Support |
| -------- | ------- |
| Relay  | :white_check_mark:    |
| Publisher | :white_check_mark:     |
| Subscriber    | :white_check_mark:   |

---

## 新功能：BBR 拥塞控制与统计日志

本项目已集成 BBRv1、BBRv3 和 Cubic 三种拥塞控制算法，并提供统计日志功能用于性能监控。

### 支持的拥塞控制算法

| 算法 | 说明 | 状态 |
|------|------|------|
| BBRv3 | BBR 第三个版本，针对高带宽延迟产品优化 | ✅ 支持 |
| BBRv1 | 原始 BBR 算法 | ✅ 支持 |
| Cubic | TCP Cubic 拥塞控制 | ✅ 支持 |

---

## 快速开始

### 1. 配置证书

```bash
cd moq-go
make cert
```

### 2. 构建项目

```bash
# 构建 relay
make relaysource

# 构建 publisher
make pubsource

# 构建 subscriber
make subsource
```

### 3. 运行测试

需要**三个终端**分别运行以下命令：

**终端 1 - 启动 Relay：**
```bash
cd moq-go
go run examples/relay/relay.go
```

**终端 2 - 启动 Subscriber（订阅者）：**
```bash
# 使用 BBRv3（默认）
make sub

# 或指定拥塞控制算法和启用统计
cd examples/newsub
go run newsub.go -congestion=bbr3 -stats

# 其他选项：
# -congestion=bbr1   # 使用 BBRv1
# -congestion=cubic   # 使用 Cubic
# -stats              # 启用统计日志
# -debug              # 启用调试日志
```

**终端 3 - 启动 Publisher（发布者）：**
```bash
# 使用 BBRv3（默认）
make pub

# 或指定拥塞控制算法和启用统计
cd examples/newpub
go run newpub.go -congestion=bbr3 -stats

# 其他选项：
# -congestion=bbr1   # 使用 BBRv1
# -congestion=cubic   # 使用 Cubic
# -stats              # 启用统计日志
# -debug              # 启用调试日志
```

---

## 命令行参数

### newpub / newsub 参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-debug` | bool | false | 启用调试日志 |
| `-stats` | bool | false | 启用统计日志输出 |
| `-congestion` | string | "bbr3" | 拥塞控制算法：cubic / bbr1 / bbr3 |

### 使用示例

```bash
# BBRv1 测试（带统计）
cd examples/newsub
go run newsub.go -congestion=bbr1 -stats

cd examples/newpub
go run newpub.go -congestion=bbr1 -stats

# Cubic 测试（带统计）
cd examples/newsub
go run newsub.go -congestion=cubic -stats

cd examples/newpub
go run newpub.go -congestion=cubic -stats

# BBRv3 测试（带统计）
cd examples/newsub
go run newsub.go -congestion=bbr3 -stats

cd examples/newpub
go run newpub.go -congestion=bbr3 -stats
```

### 日志输出

启用 `-stats` 后，输出两类日志：

**QUIC 连接统计** `[QUIC Stats]`：
```
[QUIC Stats] MinRTT=1.234ms LatestRTT=2.345ms SmoothedRTT=2.100ms PacketsSent=100 PacketsLost=0
```

**拥塞控制统计** `[Congestion Stats]`：
```
[Congestion Stats] CWND=131.07 KB PacingRate=524.29 KB/s BytesInFlight=40.95 KB State=Probe_BW ...
```

---

## API 集成指南

### 1. 配置拥塞控制算法

```go
import "github.com/DineshAdhi/moq-go/moqt"
import "github.com/quic-go/quic-go"

options := moqt.DialerOptions{
    ALPNs: []string{"moq-00"},
    QuicConfig: &quic.Config{
        EnableDatagrams: true,
        MaxIdleTimeout:  60 * time.Second,
        Congestion: func() quic.SendAlgorithmWithDebugInfos {
            return quic.NewBBRv3(nil)  // BBRv3
            // 或 quic.NewBBRv1(nil)     // BBRv1
            // 或 quic.NewCubic(nil)     // Cubic
        },
    },
}
```

### 2. 启用统计日志

```go
import "github.com/DineshAdhi/moq-go/moqt"

pub := api.NewMOQPub(options, "127.0.0.1:4443")
handler, err := pub.Connect()

// 启用统计日志（每 1 秒输出一次）
statsLogger := moqt.NewStatsLogger(pub, 1*time.Second)
statsLogger.Start()

// 运行 30 秒后停止
time.Sleep(30 * time.Second)
statsLogger.Stop()
```

---

## 接口变更说明

### 新增函数（quic-go/interface.go）

```go
// 创建 BBRv1 拥塞控制算法
func NewBBRv1(conf *Config) SendAlgorithmWithDebugInfos

// 创建 BBRv3 拥塞控制算法
func NewBBRv3(conf *Config) SendAlgorithmWithDebugInfos

// 创建 Cubic 拥塞控制算法
func NewCubic(conf *Config) SendAlgorithmWithDebugInfos
```

### 新增统计模块（moqt/stats.go）

```go
// 创建连接统计日志器
func NewStatsLogger(provider ConnectionStatsProvider, interval time.Duration) *StatsLogger

// 格式化字节数
func FormatBytes(b uint64) string

// 格式化带宽
func FormatBandwidth(bps uint64) string
```

### 新增方法

```go
// MOQTSession
func (s *MOQTSession) GetConnectionStats() ConnectionStats

// MOQPub / MOQSub
func (pub *MOQPub) GetConnectionStats() ConnectionStats
func (sub *MOQSub) GetConnectionStats() ConnectionStats
```

### Bug 修复

1. **BBRv3 HasPacingBudget 死锁问题**
   - 文件：`quic-go-bbr/internal/congestion/bbrv3.go`
   - 修复：`HasPacingBudget()` 改为 `return true`

2. **CubicSender nil 指针问题**
   - 文件：`quic-go-bbr/internal/congestion/cubic_sender.go`
   - 修复：`BandwidthEstimate()`、`MaybeExitSlowStart()`、`maybeIncreaseCwnd()` 添加 nil 检查

---

## 项目结构

```
moq-go/
├── Makefile                 # 构建命令
├── moqt/
│   ├── stats.go            # 统计日志模块
│   ├── moqtsession.go      # MOQ 会话管理
│   ├── moqtdialer.go       # MOQ 拨号器（默认 BBRv3）
│   └── api/
│       ├── pub.go          # 发布者 API
│       └── sub.go          # 订阅者 API
├── examples/
│   ├── relay/              # Relay 示例
│   ├── pub/                # Publisher 示例（make pub）
│   ├── sub/                # Subscriber 示例（make sub）
│   ├── newpub/             # Publisher 示例（支持 -stats -congestion）
│   └── newsub/             # Subscriber 示例（支持 -stats -congestion）
├── docs/
│   ├── TEST_GUIDE.md       # 测试指导文档
│   └── SECONDARY_DEVELOPMENT_GUIDE.md  # 二次开发文档
└── quic-go/                # QUIC 协议实现（集成 BBR）
    ├── interface.go        # 拥塞控制接口
    └── internal/congestion/
        ├── bbrv3.go       # BBRv3 实现
        ├── bbrv1.go       # BBRv1 实现
        └── cubic_sender.go # Cubic 实现
```

---

## Makefile 命令

| 命令 | 说明 |
|------|------|
| `make cert` | 生成自签名证书 |
| `make relaysource` | 构建 relay |
| `make pubsource` | 构建 publisher |
| `make subsource` | 构建 subscriber |
| `make relay` | 运行 relay |
| `make pub` | 运行 publisher（默认） |
| `make sub` | 运行 subscriber（默认） |

---

## 详细文档

- [测试指导文档](docs/TEST_GUIDE.md) - 测试环境搭建、测试用例、测试流程
- [二次开发文档](docs/SECONDARY_DEVELOPMENT_GUIDE.md) - API 接口、集成示例、错误处理

---