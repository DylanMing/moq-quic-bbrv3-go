# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go implementation of **MOQT (Media Over QUIC Transport)** protocol, compliant with [DRAFT04](https://dataObjectStreamer.ietf.org/doc/draft-ietf-moq-transport/04/). The library supports WebTransport and QUIC protocols with three roles: Relay, Publisher, and Subscriber.

## Common Commands

```bash
# Generate self-signed certificates (required before running examples)
make cert

# Run example implementations
make relay    # Start a relay server
make pub      # Start a publisher
make sub      # Start a subscriber

# Build examples without running
make relaysource
make pubsource
make subsource

# Clean binaries
make cleanrelay cleanpub cleansub
```

## Architecture

The codebase is organized into several packages:

- **`moqt/`** - Core MOQT protocol implementation
  - `handler.go` - Factory for creating role-specific handlers (RelayHandler, PubHandler, SubHandler)
  - `moqtsession.go` - Main session management
  - `moqtlistener.go` / `moqtdialer.go` - Listener and dialer for connections
  - `*handler.go` - Role-specific message handlers
  - `*stream.go` - Stream management for pub/sub

- **`moqt/api/`** - Public API for clients
  - `pub.go` - Publisher API (`MOQPub`)
  - `sub.go` - Subscriber API (`MOQSub`)
  - `relay.go` - Relay API (`MOQRelay`)

- **`moqt/wire/`** - Wire protocol message types (MOQT messages like Subscribe, Announce, etc.)

- **`h3/`** - HTTP/3 frame handling

- **`wt/`** - WebTransport session support

- **`examples/`** - Working implementations demonstrating Relay, Publisher, and Subscriber roles
