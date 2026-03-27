# MOQ-GO BBR 拥塞控制测试指导文档

## 文档信息

| 项目 | 内容 |
|------|------|
| 项目名称 | MOQ-GO BBR 拥塞控制集成测试 |
| 文档版本 | v1.0 |
| 创建日期 | 2026-03-27 |
| 测试对象 | BBRv1、BBRv3、Cubic 拥塞控制算法 |

---

## 1. 测试概述

### 1.1 测试目的

本测试旨在验证 moq-go 项目中集成的三种拥塞控制算法（BBRv1、BBRv3、Cubic）的功能正确性和性能表现，确保统计日志功能能够正常工作。

### 1.2 测试范围

- BBRv1 拥塞控制功能测试
- BBRv3 拥塞控制功能测试
- Cubic 拥塞控制功能测试
- 统计日志功能测试
- 三种算法的对比测试

---

## 2. 测试环境搭建

### 2.1 系统要求

| 项目 | 最低要求 | 推荐配置 |
|------|----------|----------|
| 操作系统 | Windows 10 / Linux / macOS | Windows 10 / Linux |
| Go 版本 | Go 1.21+ | Go 1.21+ |
| 内存 | 4GB | 8GB+ |
| 网络 | 本地回环网络 | 本地回环网络 |

### 2.2 项目依赖

```
github.com/quic-go/quic-go        # QUIC 协议实现（已集成 BBR）
github.com/DineshAdhi/moq-go      # MOQ 应用层协议
github.com/rs/zerolog             # 日志框架
github.com/google/uuid            # UUID 生成
golang.org/x/net/context          # Context 扩展
```

### 2.3 环境搭建步骤

#### 步骤 1：克隆项目

```bash
git clone https://github.com/DineshAdhi/moq-go.git
cd moq-go
```

#### 步骤 2：安装依赖

```bash
go mod tidy
```

#### 步骤 3：验证项目结构

```
moq-go/
├── moqt/
│   ├── stats.go           # 统计日志模块
│   ├── moqtsession.go     # MOQ 会话管理
│   ├── moqtdialer.go      # MOQ 拨号器
│   └── api/
│       ├── pub.go         # 发布者 API
│       └── sub.go         # 订阅者 API
├── examples/
│   ├── newpub/            # 发布者示例
│   │   └── newpub.go
│   └── newsub/            # 订阅者示例
│       └── newsub.go
└── quic-go/               # QUIC 协议实现（包含 BBR）
```

---

## 3. 测试用例设计

### 3.1 功能测试用例

#### TC-001：BBRv1 拥塞控制测试

| 项目 | 内容 |
|------|------|
| 用例编号 | TC-001 |
| 用例名称 | BBRv1 拥塞控制功能测试 |
| 前置条件 | newsub 已启动并连接到 newpub |
| 测试步骤 | 1. 启动 newsub（指定 `-congestion=bbr1 -stats`）<br>2. 启动 newpub（指定 `-congestion=bbr1 -stats`）<br>3. 观察日志输出 |
| 预期结果 | 1. 连接建立成功<br>2. 日志显示 `[QUIC Stats]` 和 `[Congestion Stats]`<br>3. CWND 稳定增长 |
| 测试通过标准 | 无 panic，日志正常输出 |

#### TC-002：BBRv3 拥塞控制测试

| 项目 | 内容 |
|------|------|
| 用例编号 | TC-002 |
| 用例名称 | BBRv3 拥塞控制功能测试 |
| 前置条件 | newsub 已启动并连接到 newpub |
| 测试步骤 | 1. 启动 newsub（指定 `-congestion=bbr3 -stats`）<br>2. 启动 newpub（指定 `-congestion=bbr3 -stats`）<br>3. 观察日志输出 |
| 预期结果 | 1. 连接建立成功<br>2. 日志显示 BBRv3 状态转换<br>3. 无死锁或超时 |
| 测试通过标准 | 无死锁，日志正常输出 |

#### TC-003：Cubic 拥塞控制测试

| 项目 | 内容 |
|------|------|
| 用例编号 | TC-003 |
| 用例名称 | Cubic 拥塞控制功能测试 |
| 前置条件 | newsub 已启动并连接到 newpub |
| 测试步骤 | 1. 启动 newsub（指定 `-congestion=cubic -stats`）<br>2. 启动 newpub（指定 `-congestion=cubic -stats`）<br>3. 观察日志输出 |
| 预期结果 | 1. 连接建立成功<br>2. 日志显示 Cubic 状态（SlowStart/CongestionAvoidance）<br>3. CWND 稳定增长 |
| 测试通过标准 | 无 panic，日志正常输出 |

### 3.2 统计日志测试用例

#### TC-004：连接统计日志测试

| 项目 | 内容 |
|------|------|
| 用例编号 | TC-004 |
| 用例名称 | QUIC 连接统计日志功能测试 |
| 前置条件 | newpub/newsub 已启动 |
| 测试步骤 | 1. 启动 newpub/newsub（指定 `-stats=true`）<br>2. 等待 5 秒观察日志 |
| 预期结果 | 日志包含：<br>- MinRTT、LatestRTT、SmoothedRTT<br>- PacketsSent、PacketsLost |
| 测试通过标准 | 日志每 1 秒输出一次 |

#### TC-005：拥塞统计日志测试

| 项目 | 内容 |
|------|------|
| 用例编号 | TC-005 |
| 用例名称 | 拥塞控制统计日志功能测试 |
| 前置条件 | newpub/newsub 已启动并传输数据 |
| 测试步骤 | 1. 启动 newpub/newsub（指定 `-stats=true`）<br>2. 等待数据传输<br>3. 观察拥塞日志 |
| 预期结果 | 日志包含：<br>- CWND（拥塞窗口）<br>- BytesInFlight（飞行字节）<br>- State（状态）<br>- InSlowStart、InRecovery |
| 测试通过标准 | 日志显示拥塞状态变化 |

---

## 4. 测试流程

### 4.1 测试准备

1. 打开两个终端窗口
2. 终端 A：进入 newsub 目录
3. 终端 B：进入 newpub 目录

### 4.2 测试执行

#### 测试 1：BBRv1 测试

**终端 A（订阅者）：**
```bash
cd moq-go/examples/newsub
go run newsub.go -congestion=bbr1 -stats
```

**终端 B（发布者）：**
```bash
cd moq-go/examples/newpub
go run newpub.go -congestion=bbr1 -stats
```

#### 测试 2：BBRv3 测试

**终端 A（订阅者）：**
```bash
cd moq-go/examples/newsub
go run newsub.go -congestion=bbr3 -stats
```

**终端 B（发布者）：**
```bash
cd moq-go/examples/newpub
go run newpub.go -congestion=bbr3 -stats
```

#### 测试 3：Cubic 测试

**终端 A（订阅者）：**
```bash
cd moq-go/examples/newsub
go run newsub.go -congestion=cubic -stats
```

**终端 B（发布者）：**
```bash
cd moq-go/examples/newpub
go run newpub.go -congestion=cubic -stats
```

### 4.3 测试观察

观察以下指标：

1. **连接建立**：是否在 5 秒内建立连接
2. **数据传输**：是否正常传输数据
3. **日志输出**：
   - `[QUIC Stats]` - 连接统计
   - `[Congestion Stats]` - 拥塞控制统计
4. **状态转换**：观察 BBR 状态机转换
5. **错误信息**：检查是否有 error 或 panic

---

## 5. 预期结果

### 5.1 BBRv1 测试结果

```
[QUIC Stats] MinRTT=1.234ms LatestRTT=2.345ms SmoothedRTT=2.100ms PacketsSent=100 PacketsLost=0
[Congestion Stats] CWND=131.07 KB PacingRate=524.29 KB/s BytesInFlight=40.95 KB State=... ...
```

### 5.2 BBRv3 测试结果

```
[Congestion Stats] CWND=... State=Startup (或 Drain/Probe_BW/Probe_RTT) ...
```

### 5.3 Cubic 测试结果

```
[Congestion Stats] CWND=40.96 KB State=SlowStart InSlowStart=true
[Congestion Stats] CWND=131.07 KB State=CongestionAvoidance InSlowStart=false
```

---

## 6. 缺陷报告规范

### 6.1 缺陷严重等级

| 等级 | 描述 | 示例 |
|------|------|------|
| P0 | 严重 - 导致系统崩溃 | Panic、死锁、内存溢出 |
| P1 | 高 - 导致功能不可用 | 连接建立失败、数据传输中断 |
| P2 | 中 - 功能部分受损 | 统计日志丢失部分数据 |
| P3 | 低 - 轻微问题 | 日志格式不规范 |

### 6.2 缺陷报告模板

```markdown
## 缺陷报告

**缺陷编号**：BUG-001
**测试用例**：TC-002
**严重等级**：P0
**报告日期**：2026-03-27
**测试环境**：Windows 10, Go 1.21
**测试人员**：xxx

### 缺陷描述
[详细描述缺陷现象]

### 复现步骤
1. 启动 newsub -congestion=bbr3 -stats
2. 启动 newpub -congestion=bbr3 -stats
3. 等待 30 秒

### 实际结果
[描述实际观察到的现象]

### 预期结果
[描述预期应该发生的现象]

### 日志信息
```
[粘贴相关日志]
```

### 分析结论
[分析缺陷原因]
```

---

## 7. 测试工具使用方法

### 7.1 命令行参数说明

#### newpub 参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-debug` | bool | false | 启用调试日志 |
| `-stats` | bool | false | 启用统计日志 |
| `-congestion` | string | "bbr3" | 拥塞控制算法：cubic/bbr1/bbr3 |

#### newsub 参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-debug` | bool | false | 启用调试日志 |
| `-stats` | bool | false | 启用统计日志 |
| `-congestion` | string | "bbr3" | 拥塞控制算法：cubic/bbr1/bbr3 |

### 7.2 日志分析工具

建议使用 `grep` 过滤日志：

```bash
# 查看 QUIC 统计
go run newpub.go -stats 2>&1 | findstr "QUIC Stats"

# 查看拥塞统计
go run newpub.go -stats 2>&1 | findstr "Congestion Stats"

# 查看错误信息
go run newpub.go -debug 2>&1 | findstr "Error"
```

---

## 8. 注意事项

1. **超时设置**：默认 30 秒后自动停止统计日志
2. **网络要求**：需要本地回环网络（127.0.0.1）正常运行
3. **端口占用**：确保 4443 端口未被占用
4. **BBRv3 死锁**：已修复 HasPacingBudget 问题，确保使用最新代码
5. **Cubic 统计**：Cubic 的 PacingRate 和 MaxBandwidth 固定为 0

---

## 9. 附录

### 9.1 相关文件列表

| 文件路径 | 说明 |
|----------|------|
| `moqt/stats.go` | 统计日志模块 |
| `moqt/moqtsession.go` | MOQ 会话管理 |
| `moqt/moqtdialer.go` | MOQ 拨号器 |
| `moqt/api/pub.go` | 发布者 API |
| `moqt/api/sub.go` | 订阅者 API |
| `examples/newpub/newpub.go` | 发布者示例 |
| `examples/newsub/newsub.go` | 订阅者示例 |
| `quic-go/interface.go` | 拥塞控制接口 |
| `quic-go-bbr/internal/congestion/bbrv3.go` | BBRv3 实现 |
| `quic-go-bbr/internal/congestion/cubic_sender.go` | Cubic 实现 |

### 9.2 修改历史

| 日期 | 修改内容 | 负责人 |
|------|----------|--------|
| 2026-03-27 | 初始版本 | - |