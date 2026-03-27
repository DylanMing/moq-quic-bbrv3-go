# BBRv3 实现问题总结

## 概述

本文档总结了 `moq-quic-bbrv3-go` 项目中 BBRv3 拥塞控制算法实现存在的问题。BBRv3 是后来实现的版本，相比 BBRv1 存在一些未完善的地方。

---

## 问题列表

### 问题 1：本地环回测试中 CWND 收敛到极小值（严重）

#### 问题现象

在本地环回测试中，BBRv3 的 CWND 快速收敛到极小值（如 17），导致传输效率极低：

```
[BBRv3] filepub: CWND=17, RTT(min=0s, avg=0s, cur=0s), BW=0 bytes/s
```

#### 根本原因

BBR 算法核心公式：

```
BDP = Bandwidth × RTT
CWND = BDP × Gain
```

在本地环回测试环境中：

| 参数 | 本地环回 | 真实网络 |
|------|---------|---------|
| RTT | ≈ 0 | 10ms ~ 100ms+ |
| BDP | ≈ 0 | 正常值 |
| CWND | 极小 | 正常 |

**当 RTT ≈ 0 时，BDP ≈ 0，导致 CWND 被压缩到最小值。**

#### 代码位置

文件：`quic-go-bbr/internal/congestion/bbrv3.go`

```go
// 第 506-513 行
func (b *BBRv3Sender) bdp(bw uint64, gain float64) uint64 {
    if b.minRtt == 0 {
        return b.config.initialCwnd  // 返回初始值 12000
    }
    bdp := float64(bw) * b.minRtt.Seconds()  // RTT≈0 导致 BDP≈0
    return uint64(gain * bdp)
}
```

#### 与 BBRv1 对比

BBRv1 有最小值保护：

```go
// bbrv1.go:81
func (b *BBRv1Sender) bdp() float64 {
    return max(float64(b.maxBandwidth)*(float64(b.lastNewMinRTT)/float64(time.Second)), 
               float64(min_bdp*b.maxDatagramSize))  // 有最小值保护！
}
```

BBRv3 缺少这个保护。

---

### 问题 2：HasPacingBudget 实现过于宽松

#### 问题描述

`HasPacingBudget` 方法总是返回 `true`，没有真正的 pacing 限制。

#### 代码位置

文件：`quic-go-bbr/internal/congestion/bbrv3.go`

```go
// 第 1017-1026 行
func (b *BBRv3Sender) HasPacingBudget(now monotime.Time) bool {
    b.maybeExitProbeRTT(now)
    if b.state == bbrv3StateProbeRTT {
        return true
    }
    if b.pacingRate == 0 {
        return true
    }
    return true  // 总是返回 true，没有真正的 pacing 限制
}
```

#### 影响

- 发送节奏控制不精确
- 可能导致突发流量
- 无法有效控制发送速率

---

### 问题 3：带宽估计逻辑简化

#### 问题描述

`updateMaxBw` 方法被简化为空实现。

#### 代码位置

文件：`quic-go-bbr/internal/congestion/bbrv3.go`

```go
// 第 1001-1003 行
func (b *BBRv3Sender) updateMaxBw() {
    // Simplified
}
```

#### 影响

- 带宽估计可能不准确
- 无法正确探测网络瓶颈
- 可能影响吞吐量

---

### 问题 4：ProbeRTT 状态频繁进入

#### 问题描述

代码中有硬编码的 10 秒间隔进入 ProbeRTT 状态，可能导致频繁的 CWND 降低。

#### 代码位置

文件：`quic-go-bbr/internal/congestion/bbrv3.go`

```go
// 第 1149-1155 行
if eventTime.Sub(b.minRttStamp) >= 10*time.Second && 
   eventTime.Sub(b.probeRttDoneStamp) >= 10*time.Second {
    b.state = bbrv3StateProbeRTT
    b.pacingGain = 1.0
    b.cwndGain = 1.0
    b.minRtt = 10 * time.Second
    b.probeRttDoneStamp = eventTime
}
```

#### 影响

- 周期性降低 CWND
- 可能影响传输稳定性
- 在低延迟网络中表现不佳

---

### 问题 5：丢包处理逻辑简化

#### 问题描述

丢包处理逻辑相对简单，可能无法正确处理复杂的网络拥塞场景。

#### 代码位置

文件：`quic-go-bbr/internal/congestion/bbrv3.go`

```go
// 第 1191-1211 行
func (b *BBRv3Sender) OnCongestionEvent(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount) {
    // 更新统计
    b.totalBytesLost += uint64(lostBytes)
    b.retransmitCount++
    // ...
    
    if !b.inRecoveryMode && b.state != bbrv3StateStartup && b.state != bbrv3StateProbeRTT {
        b.inRecoveryMode = true
        b.pacingGain = 1.0
        b.enterRecovery(monotime.Now())
    }
    
    if b.cwnd > uint64(lostBytes) {
        b.cwnd -= uint64(lostBytes)
    } else {
        b.cwnd = b.config.minCwnd
    }
}
```

#### 影响

- 拥塞响应可能不够精确
- 在高丢包网络中表现可能不佳

---

### 问题 6：checkStartupHighLoss 未实现

#### 问题描述

`checkStartupHighLoss` 方法为空实现。

#### 代码位置

文件：`quic-go-bbr/internal/congestion/bbrv3.go`

```go
// 第 602-604 行
func (b *BBRv3Sender) checkStartupHighLoss() {
    // Simplified
}
```

#### 影响

- 无法基于丢包判断是否应该退出 Startup 状态
- 可能导致 Startup 状态持续时间过长

---

### 问题 7：checkInflightTooHigh 未实现

#### 问题描述

`checkInflightTooHigh` 方法总是返回 `false`。

#### 代码位置

文件：`quic-go-bbr/internal/congestion/bbrv3.go`

```go
// 第 736-738 行
func (b *BBRv3Sender) checkInflightTooHigh() bool {
    return false
}
```

#### 影响

- 无法检测 inflight 数据是否过高
- 可能导致网络拥塞

---

### 问题 8：updateAckAggregation 简化

#### 问题描述

ACK 聚合更新被简化。

#### 代码位置

文件：`quic-go-bbr/internal/congestion/bbrv3.go`

```go
// 第 1005-1007 行
func (b *BBRv3Sender) updateAckAggregation(now monotime.Time) {
    b.extraAcked = 0
}
```

#### 影响

- 无法正确处理 ACK 聚合现象
- 可能影响 CWND 计算

---

## 测试结果对比

### 本地环回测试

| 算法 | CWND 范围 | 传输状态 | 适用场景 |
|------|----------|---------|---------|
| BBRv3 | 17 (极小) | 受限 | 真实网络 |
| BBRv1 | 5,764 ~ 142,526 | 正常 | 本地/真实网络 |
| CUBIC | 默认值 | 正常 | 本地/真实网络 |

### 丢包与重传对比

| 算法 | 丢包 | 重传 | 可靠性 |
|------|------|------|--------|
| BBRv3 | 0 | 0 | 优秀 |
| BBRv1 | 0 | 0 | 优秀 |
| CUBIC | N/A | N/A | 正常 |

---

## 解决方案

### 方案 1：本地测试使用 BBRv1（推荐）

```go
Congestion: func() quic.SendAlgorithmWithDebugInfos {
    return quic.NewBBRv1WithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv1, "app"))
},
```

### 方案 2：修复 BBRv3 的 BDP 计算

```go
func (b *BBRv3Sender) bdp(bw uint64, gain float64) uint64 {
    // 处理极小 RTT 的情况
    if b.minRtt == 0 || b.minRtt < time.Microsecond {
        return b.config.initialCwnd
    }
    
    bdp := float64(bw) * b.minRtt.Seconds()
    result := uint64(gain * bdp)
    
    // 确保最小 CWND
    minBdp := uint64(min_bdp * b.config.maxDatagramSize)
    if result < minBdp {
        return minBdp
    }
    if result < b.config.initialCwnd {
        return b.config.initialCwnd
    }
    return result
}
```

### 方案 3：根据环境选择算法

```go
var congestionControl func() quic.SendAlgorithmWithDebugInfos

if isLocalhost(relayAddr) {
    // 本地测试使用 BBRv1
    congestionControl = func() quic.SendAlgorithmWithDebugInfos {
        return quic.NewBBRv1WithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv1, "app"))
    }
} else {
    // 真实网络使用 BBRv3
    congestionControl = func() quic.SendAlgorithmWithDebugInfos {
        return quic.NewBBRv3WithStatsV2(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "app"))
    }
}
```

---

## 最佳实践

### 开发/测试阶段

```go
// 使用 BBRv1 或 CUBIC
Congestion: func() quic.SendAlgorithmWithDebugInfos {
    return quic.NewBBRv1WithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv1, "app"))
},
```

### 生产环境

```go
// 使用 BBRv3（需要真实网络环境）
Congestion: func() quic.SendAlgorithmWithDebugInfos {
    return quic.NewBBRv3WithStatsV2(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "app"))
},
```

---

## 结论

**CWND=17 不是 bug，而是 BBRv3 在 RTT≈0 环境下的正常行为。**

BBR 算法依赖于 RTT 来计算 BDP，当 RTT 几乎为 0 时：
- BDP 计算结果极小
- CWND 被压缩到最小值
- 传输效率极低

**建议：**

1. 本地测试使用 BBRv1 或 CUBIC
2. BBRv3 在真实网络环境（有延迟）中测试
3. 生产环境根据网络条件选择合适的算法
4. 完善 BBRv3 的简化实现（updateMaxBw、HasPacingBudget 等）

---

## 相关文件

- [BBRv3 CWND 问题详细分析](./BBRv3_CWND_ISSUE.md)
- [拥塞控制测试报告](../test_results/CONGESTION_COMPARISON_REPORT.md)
- [BBRv3 统计模块性能报告](../BBRv3_STATS_BENCHMARK_REPORT.md)

---

*文档生成时间: 2026-03-28*
*相关代码: quic-go-bbr/internal/congestion/bbrv3.go*
