# Conn - 增强版网络连接库

[![Go Report Card](https://goreportcard.com/badge/github.com/vistone/conn)](https://goreportcard.com/report/github.com/vistone/conn)
[![GoDoc](https://pkg.go.dev/badge/github.com/vistone/conn?utm_source=godoc)](https://pkg.go.dev/github.com/vistone/conn)
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
go get github.com/vistone/conn
```

## Go版本要求

使用此库需要Go 1.25.4或更高版本。

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

// 访问底层连接
rawConn := conn.GetConn()

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

## 许可证

该项目采用BSD 3-Clause许可证 - 详情请见[LICENSE](LICENSE)文件。

## 贡献

欢迎贡献！请随时提交Pull Request。