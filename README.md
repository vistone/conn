# Conn - Enhanced Network Connection Library

[![Go Report Card](https://goreportcard.com/badge/github.com/vistone/conn)](https://goreportcard.com/report/github.com/vistone/conn)
[![GoDoc](https://pkg.go.dev/badge/github.com/vistone/conn?utm_source=godoc)](https://pkg.go.dev/github.com/vistone/conn)
[![Version](https://img.shields.io/badge/version-1.0.2-blue.svg)](https://github.com/vistone/conn)
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
# Install latest version
go get github.com/vistone/conn

# Install specific version (v1.0.2)
go get github.com/vistone/conn@v1.0.2
```

## Version

Current version: **v1.0.2**

## Go Version Requirement

Go 1.25.4 or higher is required to use this library.

### Go Version Compatibility

This library is tested and compatible with:
- Go 1.25.4+ (recommended)
- Go 1.21+ (minimum required for `min()` function support)

**Important Notes:**
- The library uses Go's built-in `min()` function which was introduced in Go 1.21
- If you're using Go 1.20 or earlier, you'll need to upgrade to at least Go 1.21
- For best performance and latest features, we recommend using Go 1.25.4 or later

### Upgrading Go Version

If you need to upgrade your Go version:

```bash
# Check current Go version
go version

# Download and install latest Go from https://go.dev/dl/
# Or use your system's package manager

# Verify installation
go version
```

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

// Reset rate limiter
limiter.Reset()
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

// Reset statistics
conn.Reset()

// Access underlying connection
rawConn := conn.GetConn()

// Get rate limiter
rate := conn.GetRate()

// Check connection type
isTCP := conn.IsTCP()
isUDP := conn.IsUDP()
networkType := conn.NetworkType()

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

## Performance Benchmarks

The library has been extensively stress-tested under high-load conditions. Below are the performance test results:

### High Concurrency Test
- **Concurrent Connections**: 200
- **Operations per Connection**: 20
- **Total Throughput**: ~260-330 MB/s
- **Average Latency**: ~12-16µs per operation
- **Total Data Transferred**: ~15.6 MB (7.81 MB RX + 7.81 MB TX)
- **Test Duration**: ~50-60ms

### High Throughput Test
- **Concurrent Streams**: 10
- **Data per Stream**: 10 MB
- **Total Data**: 100 MB
- **Throughput**: ~350-620 MB/s (varies by system load)
- **Test Duration**: ~320-570ms

### Rate Limiter Performance
- **Concurrency**: 20-50 goroutines
- **Operations**: 500-1000 per goroutine
- **Token Bucket Algorithm**: Efficient token distribution with minimal overhead
- **Thread Safety**: Full atomic operations for concurrent access

### Intense Long-Duration Stress Tests

#### Extreme Rate Limiter Stress Test
- **Concurrency**: 1000 goroutines
- **Operations per Goroutine**: 10,000
- **Bytes per Operation**: 64 bytes
- **Rate Limits**: 1 GB/s read and write
- **Test Duration**: 3 minutes
- **Results**: 
  - Completed 20 million operations in 5.5 seconds
  - Operation rate: 3.6 million ops/s
  - Actual throughput: 0.22 GB/s
  - Error rate: 0%
  
#### Extreme Statistics Stress Test
- **Concurrency**: 800 goroutines
- **Operations per Goroutine**: 50,000
- **Data Size per Operation**: 256 bytes
- **Stat Groups**: 100 separate statistic groups
- **Test Duration**: 4 minutes
- **Operation Types**: Mixed read, write, query, and reset operations

#### Long Duration Comprehensive Stress Test
- **Concurrent Connections**: 500
- **Data per Connection**: 10 MB
- **Chunk Size**: 4 KB
- **Test Duration**: 5 minutes
- **Rate Limits**: 100 MB/s read and write
- **Operation Types**: Mixed read/write with echo operations

### Key Performance Characteristics
- **Low Latency**: Sub-millisecond operation overhead
- **High Throughput**: Capable of handling 600+ MB/s on modern hardware
- **Scalability**: Tested with 200+ concurrent connections
- **Memory Efficiency**: Minimal memory overhead with atomic operations
- **Thread Safety**: Zero race conditions under high concurrency

### Running Benchmarks

To run the stress tests yourself:

```bash
# Run all stress tests
go test -run TestStress -v -timeout 60s

# Run specific benchmark
go test -bench=BenchmarkStatConn -benchmem -benchtime=3s

# Run high concurrency test
go test -run TestStress_HighConcurrency -v
```

**Note**: Performance results may vary based on hardware, operating system, and system load. The above results were obtained on a Linux system with Go 1.25.4.

## License

This project is licensed under the BSD 3-Clause License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.