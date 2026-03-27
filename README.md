# moq-quic-bbrv3-go

This repository contains the moq-go implementation with BBR (Bottleneck Bandwidth and RTT) congestion control support for QUIC.

## Repository Structure

```
├── moq-go/           # MOQ (Media over QUIC) implementation
│   ├── examples/     # Example applications (pub, sub, relay)
│   │   ├── relay/    # Relay server
│   │   ├── newpub/   # Publisher example
│   │   └── newsub/   # Subscriber example
│   ├── moqt/         # MOQ protocol implementation
│   └── ...
│
└── quic-go-bbr/      # QUIC implementation with BBR support
    ├── internal/     # Internal packages
    │   └── congestion/  # Congestion control (CUBIC, BBRv1, BBRv3)
    └── ...
```

## Features

- **MOQ Protocol**: Media over QUIC implementation
- **BBR Congestion Control**: Support for BBRv1 and BBRv3 algorithms
- **CUBIC**: Default congestion control algorithm
- **Statistics Module**: Real-time connection statistics monitoring

## Congestion Control Algorithms

| Algorithm | Description |
|-----------|-------------|
| CUBIC | Default TCP congestion control |
| BBRv1 | Google BBR version 1 |
| BBRv3 | Google BBR version 3 (latest) |

## Quick Start

### 1. Start the Relay Server

```bash
cd moq-go/examples/relay
go run .
```

The relay will listen on port 4444 by default.

### 2. Start the Publisher (newpub)

```bash
cd moq-go/examples/newpub
go run .
```

### 3. Start the Subscriber (newsub)

```bash
cd moq-go/examples/newsub
go run .
```

## Testing Different Congestion Control Algorithms

### Test with BBRv3

Modify `newpub/main.go` and `newsub/main.go`:

```go
QuicConfig: &quic.Config{
    KeepAlivePeriod: 1 * time.Second,
    EnableDatagrams: true,
    MaxIdleTimeout:  60 * time.Second,
    Congestion: func() quic.SendAlgorithmWithDebugInfos {
        return quic.NewBBRv3WithStatsV2(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "newpub"))
    },
},
```

### Test with BBRv1

```go
QuicConfig: &quic.Config{
    KeepAlivePeriod: 1 * time.Second,
    EnableDatagrams: true,
    MaxIdleTimeout:  60 * time.Second,
    Congestion: func() quic.SendAlgorithmWithDebugInfos {
        return quic.NewBBRv1WithStats(nil, quic.DefaultStatsConfig(quic.AlgorithmBBRv1, "newpub"))
    },
},
```

### Test with CUBIC (Default)

```go
QuicConfig: &quic.Config{
    KeepAlivePeriod: 1 * time.Second,
    EnableDatagrams: true,
    MaxIdleTimeout:  60 * time.Second,
    Congestion: nil,  // nil means use default CUBIC
},
```

## Statistics Output

When using BBRv1 or BBRv3 with statistics, you will see output like:

```
[BBRv3] newpub: CWND=17, RTT(min=0s, avg=0s, cur=0s), BW=0 bytes/s, Sent=3470522, Lost=0, Retrans=0, State=ProbeBW
```

### Statistics Fields

| Field | Description |
|-------|-------------|
| CWND | Congestion Window size |
| RTT | Round Trip Time (min, avg, current) |
| BW | Estimated bandwidth |
| Sent | Total bytes sent |
| Lost | Total bytes lost |
| Retrans | Retransmission count |
| State | BBR state machine state |

## API Reference

### Creating BBRv3 with Statistics

```go
import "github.com/quic-go/quic-go"

config := quic.DefaultStatsConfig(quic.AlgorithmBBRv3, "myapp")
sender := quic.NewBBRv3WithStatsV2(nil, config)
```

### Creating BBRv1 with Statistics

```go
config := quic.DefaultStatsConfig(quic.AlgorithmBBRv1, "myapp")
sender := quic.NewBBRv1WithStats(nil, config)
```

## Test Reports

- [Congestion Control Comparison Report](./test_results/CONGESTION_COMPARISON_REPORT.md)
- [BBRv3 Statistics Benchmark Report](./quic-go-bbr/BBRv3_STATS_BENCHMARK_REPORT.md)
- [Statistics Usage Guide](./quic-go-bbr/STATS_USAGE_GUIDE.md)

## Requirements

- Go 1.21+
- Operating System: Windows / Linux / macOS

## License

See respective subdirectories for license information.
