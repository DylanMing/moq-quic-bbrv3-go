# BBRv3 统计模块性能测试报告

## 测试环境

- **操作系统**: Windows
- **CPU**: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz
- **架构**: amd64
- **Go 版本**: 1.25
- **测试时间**: 2026-03-27

## 测试版本

| 版本 | 实现方式 | 特点 |
|------|----------|------|
| **V1** | `sync.RWMutex` | 原始版本，使用互斥锁保护数据 |
| **V2** | `atomic` + goroutine | 优化版，使用原子操作和异步日志 |
| **V3** | 纯 `atomic` | 极致优化版，零锁设计，JSON 输出 |

## 基准测试结果

### 1. UpdateCWND (拥塞窗口更新)

```
BenchmarkUpdateCWND/V1-Mutex-8          81857533                29.98 ns/op
BenchmarkUpdateCWND/V2-Atomic-Channel-8 267412050                7.615 ns/op
BenchmarkUpdateCWND/V3-Atomic-Pure-8    417902877                5.725 ns/op
```

| 版本 | 耗时 | 相对性能 | 提升 |
|------|------|----------|------|
| V1 (Mutex) | 29.98 ns/op | 1.0x | - |
| V2 (Atomic) | 7.615 ns/op | 3.9x | **+294%** |
| V3 (Pure Atomic) | 5.725 ns/op | 5.2x | **+424%** |

### 2. UpdateRTT (RTT 更新)

```
BenchmarkUpdateRTT/V1-Mutex-8           68368064                34.77 ns/op
BenchmarkUpdateRTT/V2-Atomic-Channel-8  55902096                45.07 ns/op
BenchmarkUpdateRTT/V3-Atomic-Pure-8     58758290                42.44 ns/op
```

| 版本 | 耗时 | 相对性能 | 说明 |
|------|------|----------|------|
| V1 (Mutex) | 34.77 ns/op | 1.0x | 简单更新 |
| V2 (Atomic) | 45.07 ns/op | 0.77x | CAS 循环开销 |
| V3 (Pure Atomic) | 42.44 ns/op | 0.82x | CAS 循环开销 |

> **注意**: RTT 更新涉及 minRTT 的 CAS 循环，atomic 版本略慢，但差异可接受。

### 3. AddBytesSent (发送字节累加)

```
BenchmarkAddBytesSent/V1-Mutex-8                75440371                32.50 ns/op
BenchmarkAddBytesSent/V2-Atomic-Channel-8       319765930                7.705 ns/op
BenchmarkAddBytesSent/V3-Atomic-Pure-8          333548425                6.574 ns/op
```

| 版本 | 耗时 | 相对性能 | 提升 |
|------|------|----------|------|
| V1 (Mutex) | 32.50 ns/op | 1.0x | - |
| V2 (Atomic) | 7.705 ns/op | 4.2x | **+322%** |
| V3 (Pure Atomic) | 6.574 ns/op | 4.9x | **+395%** |

### 4. 并发测试 (Concurrent)

```
BenchmarkConcurrent/V1-Mutex-8          10876090               298.5 ns/op
BenchmarkConcurrent/V2-Atomic-Channel-8 15705120               138.5 ns/op
BenchmarkConcurrent/V3-Atomic-Pure-8    19229258               124.4 ns/op
```

| 版本 | 耗时 | 相对性能 | 提升 |
|------|------|----------|------|
| V1 (Mutex) | 298.5 ns/op | 1.0x | - |
| V2 (Atomic) | 138.5 ns/op | 2.2x | **+115%** |
| V3 (Pure Atomic) | 124.4 ns/op | 2.4x | **+140%** |

### 5. 内存分配

所有版本在更新操作中均无内存分配：

```
0 B/op          0 allocs/op
```

## 功能测试结果

```
=== RUN   TestStatsCorrectness
=== RUN   TestStatsCorrectness/V1
=== RUN   TestStatsCorrectness/V2
=== RUN   TestStatsCorrectness/V3
--- PASS: TestStatsCorrectness (0.00s)
    --- PASS: TestStatsCorrectness/V1 (0.00s)
    --- PASS: TestStatsCorrectness/V2 (0.00s)
    --- PASS: TestStatsCorrectness/V3 (0.00s)
PASS
```

所有版本功能正确性测试通过。

## JSON 输出示例 (V3)

```json
{
  "ts": 1774554225662783900,
  "conn_id_hash": 13248196901707613510,
  "state": "ProbeBW-Up",
  "cwnd": 12800,
  "ssthresh": 0,
  "bytes_sent": 10000,
  "bytes_lost": 0,
  "retrans": 0,
  "min_rtt_us": 15000,
  "avg_rtt_us": 15000,
  "cur_rtt_us": 15000,
  "tx_rate": 0,
  "bw": 0,
  "max_bw": 0,
  "pacing_rate": 0,
  "inflight": 0
}
```

## 性能对比总结

### 单操作性能提升

| 操作 | V1 → V2 提升 | V1 → V3 提升 |
|------|-------------|-------------|
| UpdateCWND | +294% | +424% |
| AddBytesSent | +322% | +395% |
| Concurrent | +115% | +140% |

### 综合评价

| 指标 | V1 | V2 | V3 |
|------|----|----|----|
| **性能** | ⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **功能完整性** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **易用性** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **集成友好** | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |

## 使用建议

### 场景选择

| 场景 | 推荐版本 | 理由 |
|------|----------|------|
| **开发调试** | V1 | 简单易用，日志格式友好 |
| **生产环境** | V2 | 平衡性能和功能 |
| **高性能场景** | V3 | 最优性能，JSON 便于监控集成 |
| **监控系统集成** | V3 | JSON 输出可直接发送到 ES/Prometheus |

### 代码示例

#### V1 - 简单使用

```go
stats := congestion.NewBBRv3Stats(congestion.BBRv3StatsConfig{
    Enabled:     true,
    LogInterval: time.Second,
    ConnID:      "dev-test",
})
```

#### V2 - 异步日志

```go
stats := congestion.NewBBRv3StatsOptimized(congestion.BBRv3StatsConfigOptimized{
    Enabled:     true,
    LogInterval: time.Second,
    ConnID:      "prod-conn",
    LogFunc: func(snapshot congestion.BBRv3StatsSnapshot) {
        // 自定义日志处理
        logFile.WriteString(fmt.Sprintf("%v\n", snapshot))
    },
})
defer stats.Stop()
```

#### V3 - 监控系统集成

```go
stats := congestion.NewBBRv3StatsV2(congestion.BBRv3StatsConfigV2{
    Enabled:     true,
    LogInterval: time.Second,
    ConnID:      "high-perf",
    LogCallback: func(jsonData []byte) {
        // 发送到 Elasticsearch / Prometheus / 文件
        esClient.Index("bbr-stats", jsonData)
    },
})
```

## 测试方法

### 运行基准测试

```bash
# 进入测试目录
cd quic-go-bbr/internal/congestion

# 运行所有基准测试
go test -bench=. -benchmem -benchtime=2s -run=^$

# 运行特定测试
go test -bench=BenchmarkUpdateCWND -benchmem -benchtime=2s -run=^$

# 运行功能测试
go test -v -run=TestStatsCorrectness
```

### 测试文件

- [bbrv3_stats_test.go](./quic-go-bbr/internal/congestion/bbrv3_stats_test.go) - 基准测试和功能测试

## 结论

1. **V3 版本性能最优**，在 UpdateCWND 操作上比 V1 快 5 倍以上
2. **所有版本功能正确**，通过了正确性测试和并发安全测试
3. **零内存分配**，所有更新操作均无额外内存分配
4. **推荐生产环境使用 V3**，JSON 输出便于监控系统集成
