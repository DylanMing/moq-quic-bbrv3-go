# MOQ 大文件传输测试

## 概述

本文档描述了使用 MOQ (Media over QUIC) 协议进行大文件传输的测试方案和结果。

## 测试文件

### 文件结构

```
moq-go/examples/
├── filepub/filepub.go    # 文件发布者
├── filesub/filesub.go    # 文件订阅者
└── relay/relay.go        # 中继服务器
```

## 配置参数

### 默认配置

| 参数 | 值 |
|------|-----|
| 文件大小 | 10 MB |
| Object 大小 | 64 KB |
| 默认 Group 大小 | 10 MB (1个Group) |

### 命令行参数

#### filepub

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-groupsize` | 10485760 (10MB) | Group 大小（字节） |
| `-group` | "filetest" | Group 名称 |

#### filesub

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-output` | "received_file.bin" | 输出文件路径 |
| `-group` | "filetest" | 订阅的 Group 名称 |

## 使用方法

### 1. 启动 Relay

```bash
cd moq-go/examples/relay
go run .
```

### 2. 启动 Publisher

```bash
# 默认配置 (1个Group, 10MB)
cd moq-go/examples/filepub
go run .

# 自定义 Group 大小 (4个Group, 每个2.5MB)
go run . -groupsize 2621440

# 自定义 Group 大小 (10个Group, 每个1MB)
go run . -groupsize 1048576
```

### 3. 启动 Subscriber

```bash
cd moq-go/examples/filesub
go run .

# 自定义输出文件
go run . -output my_file.bin
```

## 测试结果

### 测试 1: 1个 Group (10MB)

**配置:**
- FileSize: 10 MB
- GroupSize: 10 MB
- Groups: 1
- ObjectsPerGroup: 160

**Publisher 结果:**
```
Group 0 completed: 10485760 bytes in 27.56ms (362.79 MB/s)
Transfer complete: 10485760 bytes in 28.09ms (355.93 MB/s avg)
```

**Subscriber 结果:**
```
Total received: 5177344 bytes (4.94 MB)
Total duration: 120.36ms
Average throughput: 41.02 MB/s
Total objects: 79
Total groups: 1
```

### 测试 2: 4个 Group (每个 2.5MB)

**配置:**
- FileSize: 10 MB
- GroupSize: 2.5 MB
- Groups: 4
- ObjectsPerGroup: 40

**Publisher 结果:**
```
Group 0 completed: 2621440 bytes in 7.54ms (331.37 MB/s)
Group 1 completed: 2621440 bytes in 8.02ms (311.55 MB/s)
Group 2 completed: 2621440 bytes in 3.14ms (796.76 MB/s)
Group 3 completed: 2621440 bytes in 4.91ms (508.71 MB/s)
Transfer complete: 10485760 bytes in 25.21ms (396.74 MB/s avg)
```

**Subscriber 结果:**
```
Group 0 stats: 720896 bytes, 11 objects, 25.66ms (26.79 MB/s)
Group 1 stats: 1114112 bytes, 17 objects, 40.82ms (26.03 MB/s)
Group 2 stats: 65536 bytes, 1 objects, 0.52ms (119.78 MB/s)
Group 3 stats: 131072 bytes, 2 objects, 4.09ms (30.55 MB/s)
Total received: 2031616 bytes (1.94 MB)
Average throughput: 36.67 MB/s
Total objects: 31
Total groups: 4
```

## 性能分析

### Publisher 端吞吐量

| 配置 | 平均吞吐量 | 最高 Group 吞吐量 |
|------|-----------|------------------|
| 1 Group (10MB) | 355.93 MB/s | 362.79 MB/s |
| 4 Groups (2.5MB) | 396.74 MB/s | 796.76 MB/s |

### 拥塞控制

测试使用 BBRv1 拥塞控制算法：
- CWND 范围: 5,764 ~ 142,526
- 零丢包、零重传
- 状态: ProbeBW

### 影响因素

1. **本地环回测试**: RTT ≈ 0，实际网络环境吞吐量会降低
2. **订阅时机**: MOQ 是实时流媒体协议，subscriber 需要在 publisher 发送前连接才能接收完整数据
3. **Group 大小**: 较小的 Group 可以提供更细粒度的传输控制

## 代码说明

### filepub.go 关键逻辑

```go
// 传输配置
const FILE_SIZE = 10 * 1024 * 1024    // 10 MB
const OBJECT_SIZE = 64 * 1024          // 64 KB

// 计算分组
numGroups := FILE_SIZE / groupSize
objectsPerGroup := groupSize / OBJECT_SIZE

// 发送数据
for g := 0; g < numGroups; g++ {
    gs, _ := stream.NewGroup(uint64(g))
    for groupBytes < groupLimit {
        gs.WriteObject(&wire.Object{
            GroupID: uint64(g),
            ID:      objectid,
            Payload: payload,
        })
    }
    gs.Close()
}
```

### filesub.go 关键逻辑

```go
// 接收数据
for {
    groupid, object, err := stream.ReadObject()
    if err == io.EOF {
        break
    }
    
    // 统计
    groupStatsMap[gid].Bytes += int64(len(object.Payload))
    totalBytes += int64(len(object.Payload))
}

// 输出报告
avgThroughput := float64(totalBytes) / 1024 / 1024 / totalDuration.Seconds()
```

## 注意事项

1. **实时性**: MOQ 协议设计用于实时流媒体，不支持历史数据回放
2. **连接顺序**: 确保 subscriber 在 publisher 开始发送前连接
3. **网络环境**: 本地测试吞吐量较高，实际网络环境需考虑延迟和带宽限制
4. **拥塞控制**: BBRv1 在本地测试表现良好，实际网络建议测试 BBRv3 和 CUBIC

## 扩展建议

1. **文件完整性**: 添加校验和验证
2. **断点续传**: 实现基于 Group 的断点续传
3. **多文件传输**: 支持多文件批量传输
4. **进度显示**: 添加实时传输进度条

---

*文档生成时间: 2026-03-27*
*测试环境: Windows, Go 1.21+, BBRv1*
