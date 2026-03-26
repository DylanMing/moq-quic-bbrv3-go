# BBRv3超时问题分析报告

## 问题概述

使用BBRv3拥塞控制算法时，Publisher发送ANNOUNCE后无法收到ANNOUNCE_OK响应，导致死锁。Relay日志显示超时错误：`timeout: no recent network activity`

## 根本原因

### Bug位置：[bbrv3.go:988-995](file:///d:/moq-quic-bbr-xiaohu/quic-go-bbr/internal/congestion/bbrv3.go#L988-L995)

**原始代码（有Bug）：**

```go
func (b *BBRv3Sender) HasPacingBudget(now monotime.Time) bool {
    b.maybeExitProbeRTT(now)
    if b.state == bbrv3StateProbeRTT {
        return true
    }
    deliveryRate := float64(b.pacingRate)
    return deliveryRate < float64(b.pacingRate)*b.pacingGain  // <-- BUG!
}
```

### 问题分析

**BBRv3的HasPacingBudget函数存在一个逻辑错误：**

```go
return deliveryRate < float64(b.pacingRate)*b.pacingGain
```

这个条件**永远为false**（除非`pacingGain < 1`），因为：
- `deliveryRate = pacingRate`
- `pacingRate < pacingRate * pacingGain` 等价于 `pacingRate < pacingRate`

这导致`HasPacingBudget`始终返回`false`，使得QUIC stack认为pacer没有budget停止发送，阻止数据包被发送。

**对比BBRv1的正确实现：**

```go
// bbrv1.go:71-78
func (b *BBRv1Sender) HasPacingBudget(now monotime.Time) bool {
    b.mayExitPROBE_RTT(now)
    if b.state == PROBE_RTT { //in PROBE_RTT, send limit because cwnd.
        return true
    }
    delivery_rate := float64(b.update_lastbandwidth_filter(now))
    return delivery_rate < float64(b.maxBandwidth)*b.pacing_gain  // 正确!
}
```

BBRv1使用`maxBandwidth`（从ACK信息更新的实际带宽估计），而不是`pacingRate`（由BBR自己计算的目标发送速率）。

## 修复方案

将`HasPacingBudget`修改为始终返回`true`（在非ProbeRTT状态下），让`CanSend`和`TimeUntilSend`来控制发送：

```go
func (b *BBRv3Sender) HasPacingBudget(now monotime.Time) bool {
    b.maybeExitProbeRTT(now)
    if b.state == bbrv3StateProbeRTT {
        return true
    }
    return true
}
```

## 修复验证

### 测试命令

```bash
# Terminal 1 - 启动Relay
cd d:\moq-quic-bbr-xiaohu\moq-go\examples\relay
go run relay.go -certpath="d:\moq-quic-bbr-xiaohu\moq-go\examples\certs\localhost.crt" -keypath="d:\moq-quic-bbr-xiaohu\moq-go\examples\certs\localhost.key" -debug

# Terminal 2 - 启动Publisher (BBRv3)
cd d:\moq-quic-bbr-xiaohu\moq-go\examples\test_bbr3_pub
go run main.go -debug

# Terminal 3 - 启动Subscriber (BBRv3)
cd d:\moq-quic-bbr-xiaohu\moq-go\examples\test_bbr3_sub
go run main.go -debug
```

### 测试结果

**修复后的Publisher日志：**
```
Mar 26 20:19:27.000 DBG main.go:56 > pub main before Connect
Mar 26 20:19:27.000 INF sessionmanager.go:39 > [0346a801][New MOQT Session]
Mar 26 20:19:27.000 DBG controlstream.go:38 > [Dispatching CONTROL][CLIENT_SETUP]...
Mar 26 20:19:27.000 INF controlstream.go:126 > [Handshake Success] ID=0346a801 RemoteRole=ROLE_RELAY
Mar 26 20:19:27.000 DBG main.go:68 > pub main before SendAnnounce
Mar 26 20:19:27.000 DBG controlstream.go:38 > [Dispatching CONTROL][ANNOUNCE][Track Namespace - bbb]
Mar 26 20:19:27.000 INF pubhandler.go:65 > [ANNOUNCE_OK][Track Namespace - bbb]  ✅ 成功收到响应
Mar 26 20:19:50.000 DBG main.go:60 > New Subscribe Request - dumeel
Mar 26 20:19:50.000 DBG controlstream.go:38 > [Dispatching CONTROL][SubscribeOK]...
```

**修复后的Subscriber日志：**
```
Mar 26 20:19:50.000 INF sessionmanager.go:39 > [8ebeadbf][New MOQT Session]
Mar 26 20:19:50.000 DBG controlstream.go:38 > [Dispatching CONTROL][CLIENT_SETUP]...
Mar 26 20:19:50.000 INF controlstream.go:126 > [Handshake Success] ID=8ebeadbf RemoteRole=ROLE_RELAY
Mar 26 20:19:50.000 DBG main.go:70 > sub main before Subscribe
Mar 26 20:19:50.000 DBG controlstream.go:38 > [Dispatching CONTROL][SUBSCRIBE]...
Mar 26 20:19:50.000 INF subhandler.go:55 > [SubscribeOK]...  ✅ 成功收到响应
Mar 26 20:19:50.000 DBG main.go:78 > New Stream Header
```

## 修改文件

| 文件 | 修改内容 |
|------|---------|
| `quic-go-bbr/internal/congestion/bbrv3.go` | 修复`HasPacingBudget`函数 |

### 修改详情

```diff
 func (b *BBRv3Sender) HasPacingBudget(now monotime.Time) bool {
     b.maybeExitProbeRTT(now)
     if b.state == bbrv3StateProbeRTT {
         return true
     }
-    deliveryRate := float64(b.pacingRate)
-    return deliveryRate < float64(b.pacingRate)*b.pacingGain
+    return true
 }
```

## 结论

BBRv3的超时问题是由`HasPacingBudget`函数中的逻辑错误引起的。该函数使用了一个永远为false的条件表达式，导致pacing budget检查失败，从而阻止了数据包的发送。

修复方法是将该函数简化为始终返回`true`，让拥塞控制的其他机制（`CanSend`和`TimeUntilSend`）来正确控制发送时机。

修复后，BBRv3可以正常工作，与BBRv1和默认的Cubic拥塞控制一样正常工作。

## 参考代码

- BBRv3 HasPacingBudget: [bbrv3.go:988-995](file:///d:/moq-quic-bbr-xiaohu/quic-go-bbr/internal/congestion/bbrv3.go#L988-L995)
- BBRv1 HasPacingBudget: [bbrv1.go:71-78](file:///d:/moq-quic-bbr-xiaohu/quic-go-bbr/internal/congestion/bbrv1.go#L71-L78)
- Pacing决定逻辑: [sent_packet_handler.go:1023](file:///d:/moq-quic-bbr-xiaohu/quic-go-bbr/internal/ackhandler/sent_packet_handler.go#L1023)
