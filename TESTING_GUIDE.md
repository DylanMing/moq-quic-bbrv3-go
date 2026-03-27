# 拥塞控制算法对比测试指南

## 测试概述

本测试框架用于对比评估 CUBIC、BBRv1、BBRv3 三种拥塞控制算法在 QUIC/MOQ 环境下的性能表现。

## 测试指标

| 指标 | 说明 | 单位 |
|------|------|------|
| Throughput | 吞吐量 | Mbps |
| Latency | 延迟 (RTT) | ms |
| Packet Loss Rate | 丢包率 | % |
| CWND | 拥塞窗口 | bytes |
| Retransmissions | 重传次数 | count |
| Fairness | 公平性指数 | 0-1 |

## 测试步骤

### 1. 准备测试环境

```bash
# 创建测试结果目录
mkdir -p test_results

# 进入项目目录
cd d:\pack\pack
```

### 2. 测试 CUBIC 算法

```bash
# 终端 1: 启动 Relay
cd moq-go
$env:MOQT_CERT_PATH="examples/certs/localhost.crt"
$env:MOQT_KEY_PATH="examples/certs/localhost.key"
go run examples/relay/relay.go -debug

# 终端 2: 启动 newpub (修改为 CUBIC)
go run examples/newpub/newpub.go -debug 2>&1 | Tee-Object -FilePath ../test_results/CUBIC_stats.log

# 终端 3: 启动 newsub
go run examples/newsub/newsub.go -debug 2>&1 | Tee-Object -FilePath ../test_results/CUBIC_stats.log
```

### 3. 测试 BBRv1 算法

```bash
# 修改 newpub.go 和 newsub.go 使用 BBRv1
# Congestion: func() quic.SendAlgorithmWithDebugInfos {
#     return quic.NewBBRv1WithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv1, "test"))
# }

# 重复步骤 2，将输出文件改为 BBRv1_stats.log
```

### 4. 测试 BBRv3 算法

```bash
# 修改 newpub.go 和 newsub.go 使用 BBRv3
# Congestion: func() quic.SendAlgorithmWithDebugInfos {
#     return quic.NewBBRv3WithStatsV2(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "test"))
# }

# 重复步骤 2，将输出文件改为 BBRv3_stats.log
```

### 5. 分析测试数据

```bash
# 运行分析脚本
go run analyze_stats.go test_results
```

## 测试配置建议

### 网络条件模拟

可以使用 tc (Linux) 或 Clumsy (Windows) 模拟不同网络条件：

#### 低延迟环境
- 延迟: 1-10ms
- 丢包率: 0-0.1%
- 带宽: 100Mbps+

#### 高延迟环境
- 延迟: 50-200ms
- 丢包率: 0.5-2%
- 带宽: 10-50Mbps

#### 高丢包环境
- 延迟: 20-50ms
- 丢包率: 2-5%
- 带宽: 10-50Mbps

### 测试时长

| 测试类型 | 建议时长 |
|----------|----------|
| 快速测试 | 30秒 |
| 标准测试 | 2分钟 |
| 长时间测试 | 10分钟 |
| 稳定性测试 | 1小时 |

## 修改示例代码

### newpub.go 配置

```go
// CUBIC
Congestion: func() quic.SendAlgorithmWithDebugInfos {
    return nil // 使用默认 CUBIC
},

// BBRv1
Congestion: func() quic.SendAlgorithmWithDebugInfos {
    return quic.NewBBRv1WithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv1, "newpub"))
},

// BBRv3
Congestion: func() quic.SendAlgorithmWithDebugInfos {
    return quic.NewBBRv3WithStatsV2(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "newpub"))
},
```

### newsub.go 配置

```go
// 同 newpub.go 配置
```

## 公平性测试

### 多流测试

同时启动多个 publisher/subscriber 对，观察带宽分配：

```bash
# 启动多个连接
for i in {1..5}; do
    go run examples/newpub/newpub.go -debug &
    go run examples/newsub/newsub.go -debug &
done
```

### 公平性计算

使用 Jain's Fairness Index:

```
J = (Σx_i)² / (n * Σx_i²)
```

其中 x_i 是每个流的吞吐量，n 是流数量。

## 预期结果

### 吞吐量对比

| 环境 | CUBIC | BBRv1 | BBRv3 |
|------|-------|-------|-------|
| 低延迟 | 高 | 高 | 最高 |
| 高延迟 | 中 | 高 | 最高 |
| 高丢包 | 低 | 中 | 高 |

### 延迟对比

| 环境 | CUBIC | BBRv1 | BBRv3 |
|------|-------|-------|-------|
| 低延迟 | 低 | 低 | 低 |
| 高延迟 | 高 | 中 | 低 |
| 高丢包 | 高 | 中 | 低 |

### 公平性对比

| 算法 | 公平性指数 |
|------|-----------|
| CUBIC | 0.95-1.0 |
| BBRv1 | 0.85-0.95 |
| BBRv3 | 0.90-0.98 |

## 测试报告模板

测试完成后，分析脚本会自动生成报告，包含：

1. **测试环境信息**
2. **性能指标汇总表**
3. **各算法详细分析**
4. **最优算法推荐**
5. **改进建议**

## 注意事项

1. **环境一致性**: 确保每次测试的网络条件一致
2. **预热时间**: 测试前让连接稳定 5-10 秒
3. **多次测试**: 每种算法至少测试 3 次取平均值
4. **资源监控**: 监控 CPU、内存使用情况
5. **日志管理**: 及时清理旧日志文件

## 故障排除

### 问题: 连接无法建立

检查证书路径是否正确：
```bash
ls moq-go/examples/certs/
```

### 问题: 统计数据未输出

确认统计功能已启用：
```go
quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "conn-id")
// Enabled 默认为 true
```

### 问题: 数据解析失败

确保日志输出为 JSON 格式：
```go
quic.JSONStatsConfig(algorithm, connID, callback)
```
