# moq-quic-bbrv3-go

This repository contains the moq-go implementation with BBR (Bottleneck Bandwidth and RTT) congestion control support for QUIC.

## Repository Structure

```
├── moq-go/           # MOQ (Media over QUIC) implementation
│   ├── examples/     # Example applications (pub, sub, relay)
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

## Congestion Control Algorithms

| Algorithm | Description |
|-----------|-------------|
| CUBIC | Default TCP congestion control |
| BBRv1 | Google BBR version 1 |
| BBRv3 | Google BBR version 3 (latest) |

## Usage

### BBRv1

```go
QuicConfig: &quic.Config{
    Congestion: func() quic.SendAlgorithmWithDebugInfos {
        return quic.NewBBRv1(nil)
    },
}
```

### BBRv3

```go
QuicConfig: &quic.Config{
    Congestion: func() quic.SendAlgorithmWithDebugInfos {
        return quic.NewBBRv3(nil)
    },
}
```

## Test Report

See [CONGESTION_TEST_REPORT.md](./moq-go/CONGESTION_TEST_REPORT.md) for detailed test results.

## License

See respective subdirectories for license information.
