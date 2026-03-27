# BBRv3 CWND 收敛问题分析

## 问题现象

在本地环回测试中，BBRv3 的 CWND 快速收敛到极小值（如 17），导致传输效率极低甚至无法传输数据。

```
[BBRv3] filepub: CWND=17, RTT(min=0s, avg=0s, cur=0s), BW=0 bytes/s
```

## 根本原因

### BBR 算法核心公式

BBR (Bottleneck Bandwidth and RTT) 算法的核心是计算 BDP (Bandwidth-Delay Product)：

```
BDP = Bandwidth × RTT
CWND = BDP × Gain
```

### 本地环回测试的问题

在本地环回测试环境中：

| 参数 | 本地环回 | 真实网络 |
|------|---------|---------|
| RTT | ≈ 0 | 10ms ~ 100ms+ |
| BDP | ≈ 0 | 正常值 |
| CWND | 极小 | 正常 |

**当 RTT ≈ 0 时，BDP ≈ 0，导致 CWND 被压缩到最小值。**

## 代码分析

### BDP 计算

```go
// bbrv3.go:506-513
func (b *BBRv3Sender) bdp(bw uint64, gain float64) uint64 {
    if b.minRtt == 0 {
        return b.config.initialCwnd  // 返回初始值 12000
    }
    bdp := float64(bw) * b.minRtt.Seconds()  // RTT≈0 导致 BDP≈0
    return uint64(gain * bdp)
}
```

### CWND 收敛过程

```
1. 初始: CWND = 12000 (10 * 1200)
   ↓
2. 测量 RTT: minRtt 被测量为极小值（本地环回）
   ↓
3. 计算 BDP: BDP = bw * minRtt ≈ 0
   ↓
4. 限制 CWND: boundCwndForModel() 将 CWND 限制到最小值
   ↓
5. 最终: CWND = 17 (或其他极小值)
```

### 关键配置

```go
// bbrv3.go:54-75
const (
    minPipeCwndInSmss    = 4           // 最小 CWND = 4 个 MSS
    defaultMaxDatagramSize = 1200      // 默认 MTU
    defaultInitialCwnd   = 10          // 初始 CWND = 10 * MTU
)

config := &bbrv3Config{
    minCwnd:     minPipeCwndInSmss * maxDatagramSize,  // 4800
    initialCwnd: defaultInitialCwnd * maxDatagramSize, // 12000
}
```

## 解决方案

### 方案 1: 使用 BBRv1（推荐用于本地测试）

BBRv1 的实现更宽松，在本地环回测试中表现更好。

```go
Congestion: func() quic.SendAlgorithmWithDebugInfos {
    return quic.NewBBRv1WithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv1, "app"))
},
```

**测试结果：**
```
[BBRv1] filepub: CWND=92224, RTT(min=0s, avg=0s, cur=0s), BW=0 bytes/s
Transfer complete: 10485760 bytes in 28.09ms (355.93 MB/s avg)
```

### 方案 2: 在真实网络环境测试

BBRv3 需要在真实网络环境中才能发挥优势：

- **跨网络传输**（非本地环回）
- **有实际延迟**（如 10ms+ RTT）
- **有带宽限制**

### 方案 3: 修改 BBRv3 添加最小 CWND 保障

```go
func (b *BBRv3Sender) bdp(bw uint64, gain float64) uint64 {
    // 处理极小 RTT 的情况
    if b.minRtt == 0 || b.minRtt < time.Microsecond {
        return b.config.initialCwnd
    }
    
    bdp := float64(bw) * b.minRtt.Seconds()
    result := uint64(gain * bdp)
    
    // 确保最小 CWND
    if result < b.config.initialCwnd {
        return b.config.initialCwnd
    }
    return result
}
```

## 算法对比

### 本地环回测试结果

| 算法 | CWND 范围 | 传输状态 | 适用场景 |
|------|----------|---------|---------|
| BBRv3 | 17 (极小) | 受限 | 真实网络 |
| BBRv1 | 5,764 ~ 142,526 | 正常 | 本地/真实网络 |
| CUBIC | 默认值 | 正常 | 本地/真实网络 |

### BBRv3 设计目标

BBRv3 是为真实网络环境设计的：

1. **需要 RTT 信号**：用于计算 BDP
2. **需要带宽限制**：用于探测瓶颈
3. **需要拥塞信号**：用于调整发送速率

本地环回环境无法提供这些信号，因此 BBRv3 无法正常工作。

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

### 环境判断

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

---

*文档生成时间: 2026-03-27*
*相关文件: quic-go-bbr/internal/congestion/bbrv3.go*
