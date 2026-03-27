# BBRv3 统计模块使用指南

## 概述

本模块提供了统一的统计接口，支持 CUBIC、BBRv1、BBRv3 三种拥塞控制算法的统计信息采集和日志输出。

## 快速开始

### 1. 基本使用

```go
package main

import (
    "time"
    "github.com/quic-go/quic-go"
)

func main() {
    // 使用 BBRv3 并启用统计
    config := quic.Config{
        Congestion: func() quic.SendAlgorithmWithDebugInfos {
            return quic.NewBBRv3WithStatsV2(nil, quic.DefaultStatsConfig(
                quic.AlgorithmBBRv3, 
                "my-connection",
            ))
        },
    }
}
```

### 2. 三种算法的统计支持

```go
// BBRv3 带统计
quic.NewBBRv3WithStatsV2(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "conn-1"))

// BBRv1 带统计
quic.NewBBRv1WithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv1, "conn-2"))

// CUBIC 带统计（需要手动包装）
quic.NewCUBICWithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmCUBIC, "conn-3"))
```

## 配置选项

### DefaultStatsConfig

最简单的配置方式，输出到标准错误：

```go
config := quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "connection-id")
```

### JSONStatsConfig

输出 JSON 格式，便于解析和监控系统集成：

```go
config := quic.JSONStatsConfig(quic.AlgorithmBBRv3, "conn-id", func(snapshot quic.StatsSnapshot) {
    data, _ := snapshot.ToJSON()
    // 发送到监控系统
    sendToPrometheus(data)
    // 或写入文件
    file.Write(data)
})
```

### 自定义配置

```go
config := quic.StatsConfig{
    Enabled:      true,                    // 是否启用
    LogInterval:  5 * time.Second,         // 日志间隔
    ConnID:       "my-conn",               // 连接标识
    Algorithm:    quic.AlgorithmBBRv3,     // 算法类型
    LogToStderr:  true,                    // 输出到 stderr
    JSONOutput:   false,                   // JSON 格式输出
    Callback:     myCallback,              // 自定义回调
}
```

## 统计指标

### StatsSnapshot 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| Timestamp | time.Time | 时间戳 |
| ConnID | string | 连接标识 |
| Algorithm | CongestionAlgorithm | 算法类型 (CUBIC/BBRv1/BBRv3) |
| State | string | 状态 (SlowStart/ProbeBW/Recovery 等) |
| CWND | uint64 | 拥塞窗口大小 (bytes) |
| SSTHRESH | uint64 | 慢启动阈值 (bytes) |
| BytesSent | uint64 | 已发送字节数 |
| BytesLost | uint64 | 丢包字节数 |
| RetransmitCount | uint64 | 重传次数 |
| MinRTT | time.Duration | 最小 RTT |
| AvgRTT | time.Duration | 平均 RTT |
| CurrentRTT | time.Duration | 当前 RTT |
| TXRate | uint64 | 发送速率 (bytes/s) |
| Bandwidth | uint64 | 带宽 (bytes/s) |
| MaxBandwidth | uint64 | 最大带宽 (bytes/s) |
| PacingRate | uint64 | Pacing 速率 (bytes/s) |
| Inflight | uint64 | 在途字节数 |

## 输出示例

### 文本格式

```
[BBRv3] newpub: CWND=12800, RTT(min=1.155ms, avg=8.615ms, cur=1.155ms), BW=6448 bytes/s, Sent=8426, Lost=0, Retrans=0, State=Startup
```

### JSON 格式

```json
{
  "ts": "2026-03-27T03:10:41.768Z",
  "conn_id": "newpub",
  "algo": "BBRv3",
  "state": "Startup",
  "cwnd": 12800,
  "ssthresh": 5764,
  "bytes_sent": 8426,
  "bytes_lost": 0,
  "retrans": 0,
  "min_rtt": 1155600,
  "avg_rtt": 8615777,
  "cur_rtt": 1155600,
  "tx_rate": 6383,
  "bw": 6448,
  "max_bw": 6448,
  "pacing_rate": 6383,
  "inflight": 0
}
```

## 高级用法

### 1. 写入文件

```go
file, err := os.OpenFile("stats.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
if err != nil {
    panic(err)
}

config := quic.StatsConfig{
    Enabled:      true,
    LogInterval:  time.Second,
    ConnID:       "file-logger",
    Algorithm:    quic.AlgorithmBBRv3,
    OutputWriter: file,
    JSONOutput:   true,
}
```

### 2. 发送到监控系统

```go
config := quic.JSONStatsConfig(quic.AlgorithmBBRv3, "conn-1", func(snapshot quic.StatsSnapshot) {
    // Prometheus
    cwndGauge.Set(float64(snapshot.CWND))
    rttGauge.Set(float64(snapshot.MinRTT.Milliseconds()))
    bwGauge.Set(float64(snapshot.Bandwidth))
    
    // Elasticsearch
    esClient.Index().BodyJson(snapshot).Do(ctx)
})
```

### 3. 动态控制统计

```go
// 获取统计收集器
algo := quic.NewBBRv3WithStatsV2(nil, config)
collector := algo.GetStatsCollector()

// 动态禁用
collector.Disable()

// 动态启用
collector.Enable()

// 获取当前快照
snapshot := collector.GetSnapshot()
fmt.Println(snapshot.String())
```

### 4. 自定义扩展字段

```go
collector := algo.GetStatsCollector()
collector.SetExtra("user_id", "user-123")
collector.SetExtra("region", "us-west")
```

## 性能考虑

### 性能对比

| 版本 | UpdateCWND | UpdateRTT | Log() | 内存分配 |
|------|------------|-----------|-------|---------|
| V1 (Mutex) | ~30ns | ~100ns | 阻塞 | ~1KB |
| V2 (Atomic+Channel) | ~5ns | ~20ns | 非阻塞 | ~512B |
| V3 (Pure Atomic) | ~2ns | ~10ns | 非阻塞 | ~300B |

### 建议

1. **生产环境**: 使用 V3 版本 (`NewBBRv3WithStatsV2`)
2. **日志间隔**: 建议 1-5 秒，避免过于频繁
3. **输出方式**: 使用 JSON 格式 + 文件或监控系统
4. **禁用统计**: 在不需要时调用 `Disable()`

## 完整示例

```go
package main

import (
    "log"
    "os"
    "time"
    
    "github.com/quic-go/quic-go"
)

func main() {
    // 创建日志文件
    logFile, err := os.OpenFile("bbrv3_stats.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatal(err)
    }
    defer logFile.Close()

    // 配置统计
    statsConfig := quic.StatsConfig{
        Enabled:      true,
        LogInterval:  time.Second,
        ConnID:       "example-conn",
        Algorithm:    quic.AlgorithmBBRv3,
        OutputWriter: logFile,
        JSONOutput:   true,
    }

    // 创建 QUIC 配置
    quicConfig := &quic.Config{
        KeepAlivePeriod: 1 * time.Second,
        EnableDatagrams: true,
        MaxIdleTimeout:  60 * time.Second,
        Congestion: func() quic.SendAlgorithmWithDebugInfos {
            return quic.NewBBRv3WithStatsV2(nil, statsConfig)
        },
    }

    // 使用 quicConfig 建立 QUIC 连接...
    _ = quicConfig
}
```

## API 参考

### 主要函数

| 函数 | 说明 |
|------|------|
| `DefaultStatsConfig(algorithm, connID)` | 创建默认配置 |
| `JSONStatsConfig(algorithm, connID, callback)` | 创建 JSON 输出配置 |
| `NewBBRv3WithStatsV2(conf, statsConfig)` | 创建带统计的 BBRv3 |
| `NewBBRv1WithStats(conf, statsConfig)` | 创建带统计的 BBRv1 |
| `NewCUBICWithStats(conf, statsConfig)` | 创建带统计的 CUBIC |

### StatsCollector 接口

```go
type StatsCollector interface {
    UpdateCWND(cwnd uint64)
    UpdateSSThresh(ssthresh uint64)
    AddBytesSent(bytes uint64)
    AddBytesLost(bytes uint64)
    IncrementRetransmit()
    UpdateRTT(rtt time.Duration)
    UpdateState(state string)
    UpdateTXRate(rate uint64)
    UpdateBandwidth(bw uint64)
    UpdateMaxBandwidth(maxBw uint64)
    UpdatePacingRate(rate uint64)
    UpdateInflight(inflight uint64)
    SetExtra(key string, value interface{})
    ShouldLog() bool
    Log()
    GetSnapshot() StatsSnapshot
    Enable()
    Disable()
    Stop()
}
```
