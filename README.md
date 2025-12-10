# Conn - Enhanced Network Connection Library

[![Go Report Card](https://goreportcard.com/badge/github.com/vistone/conn)](https://goreportcard.com/report/github.com/vistone/conn)
[![GoDoc](https://pkg.go.dev/badge/github.com/vistone/conn?utm_source=godoc)](https://pkg.go.dev/github.com/vistone/conn)
![GitHub](https://img.shields.io/github/license/vistone/conn)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/vistone/conn)

Conn is a Go library that extends `net.Conn` with additional functionalities including timeout reading, rate limiting, statistics, and bidirectional data exchange.

## Features

- **Rate Limiting**: Token bucket algorithm for controlling read/write speed
- **Statistics**: Real-time monitoring of sent/received bytes
- **Bidirectional Data Exchange**: Efficient data forwarding between connections
- **Protocol Support**: Full support for both TCP and UDP protocols
- **Timeout Handling**: Configurable read timeouts for connections
- **Thread Safety**: Atomic operations for statistics and rate limiting

## Installation

```bash
go get github.com/vistone/conn
```

## Go Version Requirement

Go 1.25.4 or higher is required to use this library.

## Core Components

### 1. RateLimiter

Implements token bucket algorithm for rate limiting read/write operations:

```go
// Create a rate limiter (1000 bytes/second read, 2000 bytes/second write)
limiter := conn.NewRateLimiter(1000, 2000)

// Wait for read tokens
limiter.WaitRead(100)

// Wait for write tokens
limiter.WaitWrite(200)

// Dynamically adjust rates
limiter.SetRate(1500, 3000)
```

### 2. StatConn

Wraps `net.Conn` to provide byte counting and rate limiting:

```go
var rx, tx uint64
conn := conn.NewStatConn(netConn, &rx, &tx, rateLimiter)

// Read/write data normally
n, err := conn.Read(buffer)
n, err = conn.Write(data)

// Get statistics
received := conn.GetRX()
sent := conn.GetTX()
total := conn.GetTotal()

// Access underlying connection
rawConn := conn.GetConn()

// Protocol-specific operations
if tcpConn, ok := conn.AsTCPConn(); ok {
    tcpConn.SetKeepAlive(true)
}
```

### 3. TimeoutReader

Wrapper that adds timeout functionality to `net.Conn`:

```go
reader := &conn.TimeoutReader{
    Conn:    netConn,
    Timeout: 30 * time.Second,
}

n, err := reader.Read(buffer)
```

### 4. DataExchange

Facilitates bidirectional data exchange between two connections:

```go
// Exchange data between conn1 and conn2 with idle timeout
err := conn.DataExchange(conn1, conn2, 60*time.Second, buf1, buf2)
```

## Supported Operations

### TCP-Specific Operations

- `SetKeepAlive(bool)` - Enable/disable keep-alive
- `SetKeepAlivePeriod(time.Duration)` - Set keep-alive period
- `SetNoDelay(bool)` - Enable/disable Nagle's algorithm
- `SetLinger(int)` - Set linger time
- `CloseRead()` - Close read side
- `CloseWrite()` - Close write side

### UDP-Specific Operations

- `ReadFromUDP([]byte)` - Read UDP packet with sender address
- `WriteToUDP([]byte, *net.UDPAddr)` - Send UDP packet to address
- `ReadMsgUDP([]byte, []byte)` - Read UDP message with control data
- `WriteMsgUDP([]byte, []byte, *net.UDPAddr)` - Send UDP message with control data
- `SetReadBuffer(int)` - Set read buffer size
- `SetWriteBuffer(int)` - Set write buffer size

## License

This project is licensed under the BSD 3-Clause License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.