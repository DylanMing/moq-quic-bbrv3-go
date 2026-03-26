# MOQ-GO BBR拥塞控制测试报告 (newpub/newsub版本)

## 测试概述

本报告使用newpub和newsub示例测试moq-go在使用不同拥塞控制算法时的行为，包括默认拥塞控制(Cubic)、BBRv1和BBRv3。

## 测试环境

- **moq-go**: 基于 `github.com/DineshAdhi/moq-go` 的MOQ协议实现
- **quic-go-bbr**: 包含BBRv1和BBRv3拥塞控制算法的QUIC库
- **测试场景**: Pub/Sub模式下，Publisher连接Relay并发送Announce，Subscriber连接并订阅Track

## 测试结果汇总

| 拥塞控制 | Publisher | Subscriber | 结果 |
|----------|-----------|------------|------|
| **默认(Cubic)** | ✅ 连接成功 | ✅ 连接成功 | ✅ 工作正常 |
| **BBRv1** | ✅ 连接成功 | ✅ 连接成功 | ✅ 工作正常 |
| **BBRv3** | ❌ 死锁 | ✅ 连接成功 | ❌ 死锁问题 |

## 详细测试结果

### 1. 默认拥塞控制 (Cubic)

#### 测试命令
```bash
# Terminal 1 - 启动Relay
go run examples/relay/relay.go -certpath=examples/certs/localhost.crt -keypath=examples/certs/localhost.key -debug

# Terminal 2 - 启动Publisher (test_default_pub)
go run examples/test_default_pub/main.go -debug

# Terminal 3 - 启动Subscriber (test_default_sub)
go run examples/test_default_sub/main.go -debug
```

#### Publisher 日志
```
Mar 26 19:56:44.000 DBG main.go:53 > pub main before Connect
Mar 26 19:56:44.000 INF sessionmanager.go:39 > [97f3e4f1][New MOQT Session]
Mar 26 19:56:44.000 DBG controlstream.go:38 > [Dispatching CONTROL][CLIENT_SETUP]
Mar 26 19:56:44.000 INF controlstream.go:126 > [Handshake Success] ID=97f3e4f1 RemoteRole=ROLE_RELAY
Mar 26 19:56:44.000 DBG main.go:65 > pub main before SendAnnounce
Mar 26 19:56:44.000 DBG controlstream.go:38 > [Dispatching CONTROL][ANNOUNCE][Track Namespace - bbb]
Mar 26 19:57:03.000 INF pubhandler.go:65 > [ANNOUNCE_OK][Track Namespace - bbb]
Mar 26 19:57:43.000 DBG main.go:57 > New Subscribe Request - dumeel
Mar 26 19:57:43.000 DBG controlstream.go:38 > [Dispatching CONTROL][SubscribeOK]
```

**状态**: ✅ **工作正常** - ANNOUNCE/ANNOUNCE_OK交互正常，成功收到订阅请求

---

### 2. BBRv1

#### 测试命令
```bash
# Terminal 1 - 启动Relay
go run examples/relay/relay.go -certpath=examples/certs/localhost.crt -keypath=examples/certs/localhost.key -debug

# Terminal 2 - 启动Publisher (test_bbr1_pub)
go run examples/test_bbr1_pub/main.go -debug

# Terminal 3 - 启动Subscriber (test_bbr1_sub)
go run examples/test_bbr1_sub/main.go -debug
```

#### Publisher 日志
```
Mar 26 19:58:45.000 DBG main.go:56 > pub main before Connect
Mar 26 19:58:45.000 INF sessionmanager.go:39 > [266621d1][New MOQT Session]
Mar 26 19:58:45.000 DBG controlstream.go:38 > [Dispatching CONTROL][CLIENT_SETUP]
Mar 26 19:58:45.000 INF controlstream.go:126 > [Handshake Success] ID=266621d1 RemoteRole=ROLE_RELAY
Mar 26 19:58:45.000 DBG main.go:68 > pub main before SendAnnounce
Mar 26 19:58:45.000 DBG controlstream.go:38 > [Dispatching CONTROL][ANNOUNCE][Track Namespace - bbb]
Mar 26 19:58:45.000 INF pubhandler.go:65 > [ANNOUNCE_OK][Track Namespace - bbb]
Mar 26 19:59:25.000 DBG main.go:60 > New Subscribe Request - dumeel
Mar 26 19:59:25.000 DBG controlstream.go:38 > [Dispatching CONTROL][SubscribeOK]
```

**状态**: ✅ **工作正常** - ANNOUNCE/ANNOUNCE_OK交互正常，成功收到订阅请求

---

### 3. BBRv3

#### 测试命令
```bash
# Terminal 1 - 启动Relay
go run examples/relay/relay.go -certpath=examples/certs/localhost.crt -keypath=examples/certs/localhost.key -debug

# Terminal 2 - 启动Publisher (test_bbr3_pub)
go run examples/test_bbr3_pub/main.go -debug

# Terminal 3 - 启动Subscriber (test_bbr3_sub)
go run examples/test_bbr3_sub/main.go -debug
```

#### Publisher 日志
```
Mar 26 20:00:32.000 DBG main.go:56 > pub main before Connect
Mar 26 20:00:32.000 INF sessionmanager.go:39 > [da371239][New MOQT Session]
Mar 26 20:00:32.000 DBG controlstream.go:38 > [Dispatching CONTROL][CLIENT_SETUP]
Mar 26 20:00:32.000 INF controlstream.go:126 > [Handshake Success] ID=da371239 RemoteRole=ROLE_RELAY
Mar 26 20:00:32.000 DBG main.go:68 > pub main before SendAnnounce
Mar 26 20:00:32.000 DBG controlstream.go:38 > [Dispatching CONTROL][ANNOUNCE][Track Namespace - bbb]
Mar 26 20:00:32.000 DBG main.go:70 > pub main after SendAnnounce
fatal error: all goroutines are asleep - deadlock!

goroutine 1 [chan receive (nil chan)]:
main.main()
        main.go:71

goroutine 21 [chan receive]:
github.com/DineshAdhi/moq-go/moqt/api.(*MOQPub).Connect.func1()
        pub/api.go:49
```

**状态**: ❌ **死锁问题** - ANNOUNCE发送后无法收到ANNOUNCE_OK

#### Relay 日志
```
Mar 26 20:00:32.000 INF sessionmanager.go:39 > [771fc47c][New MOQT Session]
Mar 26 20:00:32.000 INF controlstream.go:101 > [Handshake Success] ID=771fc47c RemoteRole=ROLE_PUBLISHER
Mar 26 20:01:02.000 ERR relayhandler.go:123 > [Error Accepting Unistream][timeout: no recent network activity]
Mar 26 20:01:02.000 ERR moqtsession.go:78 > [Closing MOQT Session][Code - 1][Session Closed]
```

## BBRv3死锁分析

### 问题描述

BBRv3配置下，Publisher发送ANNOUNCE后，无法收到ANNOUNCE_OK响应，导致：
1. `HandleSubscribe` goroutine 永久阻塞在等待ANNOUNCE_OK
2. main goroutine 等待 `HandleSubscribe` 完成
3. 形成死锁

### 可能原因

1. **BBRv3状态机问题**: BBRv3的拥塞控制状态机可能影响了QUIC连接的数据传输
2. **计时器问题**: BBRv3可能改变了ack时钟或超时计时器
3. **流控制问题**: BBRv3的pacing算法可能影响了控制消息的发送

### 代码位置

- **发送ANNOUNCE**: `controlstream.go:38`
- **等待ANNOUNCE_OK**: `pubhandler.go:65` (`handleSubscribe`函数)

## 修改总结

### 已完成的修改

| 文件 | 修改内容 |
|------|---------|
| `quic-go-bbr/interface.go` | 添加了 `NewBBRv3()` 函数 |
| `moq-go/moqt/moqtdialer.go` | 默认使用BBRv3 |
| `moq-go/moqt/moqlistener.go` | 默认使用BBRv3 |
| `moq-go/moqt/pubhandler.go` | 改为缓冲channel (容量10) |
| `moq-go/moqt/subhandler.go` | 改为缓冲channel (容量10) |

### 测试文件

| 目录 | 配置 |
|------|------|
| `examples/test_default_pub/main.go` | 默认拥塞控制 (Cubic) |
| `examples/test_default_sub/main.go` | 默认拥塞控制 (Cubic) |
| `examples/test_bbr1_pub/main.go` | BBRv1 |
| `examples/test_bbr1_sub/main.go` | BBRv1 |
| `examples/test_bbr3_pub/main.go` | BBRv3 |
| `examples/test_bbr3_sub/main.go` | BBRv3 |

## 结论

1. **默认配置(Cubic)**: ✅ 工作正常
2. **BBRv1**: ✅ 工作正常
3. **BBRv3**: ❌ 存在死锁问题，需要进一步调查

## 建议

BBRv3死锁问题需要进一步调查，可能需要：
1. 检查BBRv3的状态机实现
2. 检查pacing算法对控制消息的影响
3. 添加更多的调试日志来追踪ANNOUNCE_OK丢失的原因