# Conn - 增强版网络连接库

[![Go Report Card](https://goreportcard.com/badge/github.com/vistone/conn)](https://goreportcard.com/report/github.com/vistone/conn)
[![GoDoc](https://pkg.go.dev/badge/github.com/vistone/conn?utm_source=godoc)](https://pkg.go.dev/github.com/vistone/conn)
[![Version](https://img.shields.io/badge/version-1.0.2-blue.svg)](https://github.com/vistone/conn)
![GitHub](https://img.shields.io/github/license/vistone/conn)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/vistone/conn)

Conn是一个Go语言库，它扩展了`net.Conn`的功能，包括超时读取、速率限制、统计和双向数据交换等功能。

## 特性

- **速率限制**：基于令牌桶算法控制读写速度
- **统计功能**：实时监控发送/接收的字节数
- **双向数据交换**：在连接之间高效转发数据
- **协议支持**：完整支持TCP和UDP协议
- **超时处理**：可配置的连接读取超时
- **线程安全**：使用原子操作进行统计和速率限制

## 安装

```bash
# 安装最新版本
go get github.com/vistone/conn

# 安装指定版本 (v1.0.2)
go get github.com/vistone/conn@v1.0.2
```

## 版本

当前版本：**v1.0.2**

## Go版本要求

使用此库需要Go 1.25.4或更高版本。

### Go版本兼容性

本库已测试并兼容以下版本：
- Go 1.25.4+（推荐）
- Go 1.21+（最低要求，支持`min()`函数）

**重要提示：**
- 本库使用了Go内置的`min()`函数，该函数在Go 1.21中引入
- 如果您使用的是Go 1.20或更早版本，需要升级到至少Go 1.21
- 为了获得最佳性能和最新功能，我们推荐使用Go 1.25.4或更高版本

### 升级Go版本

如果您需要升级Go版本：

```bash
# 检查当前Go版本
go version

# 从 https://go.dev/dl/ 下载并安装最新版Go
# 或使用系统的包管理器

# 验证安装
go version
```

## 核心组件

### 1. RateLimiter（速率限制器）

使用令牌桶算法实现读写操作的速率限制：

```go
// 创建速率限制器（每秒读取1000字节，每秒写入2000字节）
limiter := conn.NewRateLimiter(1000, 2000)

// 等待读取令牌
limiter.WaitRead(100)

// 等待写入令牌
limiter.WaitWrite(200)

// 动态调整速率
limiter.SetRate(1500, 3000)

// 重置速率限制器
limiter.Reset()
```

### 2. StatConn（统计连接）

封装`net.Conn`以提供字节计数和速率限制功能：

```go
var rx, tx uint64
conn := conn.NewStatConn(netConn, &rx, &tx, rateLimiter)

// 正常读写数据
n, err := conn.Read(buffer)
n, err = conn.Write(data)

// 获取统计数据
received := conn.GetRX()
sent := conn.GetTX()
total := conn.GetTotal()

// 重置统计数据
conn.Reset()

// 访问底层连接
rawConn := conn.GetConn()

// 获取速率限制器
rate := conn.GetRate()

// 检查连接类型
isTCP := conn.IsTCP()
isUDP := conn.IsUDP()
networkType := conn.NetworkType()

// 协议特定操作
if tcpConn, ok := conn.AsTCPConn(); ok {
    tcpConn.SetKeepAlive(true)
}
```

### 3. TimeoutReader（超时读取器）

为`net.Conn`添加超时功能的包装器：

```go
reader := &conn.TimeoutReader{
    Conn:    netConn,
    Timeout: 30 * time.Second,
}

n, err := reader.Read(buffer)
```

### 4. DataExchange（数据交换）

促进两个连接之间的双向数据交换：

```go
// 在conn1和conn2之间交换数据，带空闲超时
err := conn.DataExchange(conn1, conn2, 60*time.Second, buf1, buf2)
```

## 支持的操作

### TCP特定操作

- `SetKeepAlive(bool)` - 启用/禁用keep-alive
- `SetKeepAlivePeriod(time.Duration)` - 设置keep-alive周期
- `SetNoDelay(bool)` - 启用/禁用Nagle算法
- `SetLinger(int)` - 设置linger时间
- `CloseRead()` - 关闭读取端
- `CloseWrite()` - 关闭写入端

### UDP特定操作

- `ReadFromUDP([]byte)` - 读取带有发送方地址的UDP数据包
- `WriteToUDP([]byte, *net.UDPAddr)` - 向地址发送UDP数据包
- `ReadMsgUDP([]byte, []byte)` - 读取带有控制数据的UDP消息
- `WriteMsgUDP([]byte, []byte, *net.UDPAddr)` - 发送带有控制数据的UDP消息
- `SetReadBuffer(int)` - 设置读取缓冲区大小
- `SetWriteBuffer(int)` - 设置写入缓冲区大小

## 性能基准测试

本库已通过高强度压力测试。以下是性能测试结果：

### 高并发测试
- **并发连接数**: 200
- **每个连接操作次数**: 20
- **总吞吐量**: ~260-330 MB/s
- **平均延迟**: ~12-16µs 每次操作
- **总传输数据**: ~15.6 MB (7.81 MB 接收 + 7.81 MB 发送)
- **测试耗时**: ~50-60ms

### 高吞吐量测试
- **并发流数**: 10
- **每个流数据量**: 10 MB
- **总数据量**: 100 MB
- **吞吐量**: ~620 MB/s
- **测试耗时**: ~320ms

### 速率限制器性能
- **并发数**: 20-50 个goroutine
- **每个goroutine操作数**: 500-1000
- **令牌桶算法**: 高效的令牌分发，开销极小
- **线程安全**: 完全使用原子操作支持并发访问

### 关键性能特征
- **低延迟**: 操作开销低于毫秒级
- **高吞吐量**: 在现代硬件上可处理 600+ MB/s
- **可扩展性**: 已测试 200+ 并发连接
- **内存效率**: 使用原子操作，内存开销极小
- **线程安全**: 高并发下零竞态条件

### 运行基准测试

要自行运行压力测试：

```bash
# 运行所有压力测试
go test -run TestStress -v -timeout 60s

# 运行特定基准测试
go test -bench=BenchmarkStatConn -benchmem -benchtime=3s

# 运行高并发测试
go test -run TestStress_HighConcurrency -v
```

**注意**: 性能结果可能因硬件、操作系统和系统负载而异。上述结果在 Linux 系统上使用 Go 1.25.4 获得。

## 许可证

该项目采用BSD 3-Clause许可证 - 详情请见[LICENSE](LICENSE)文件。

## 贡献

欢迎贡献！请随时提交Pull Request。