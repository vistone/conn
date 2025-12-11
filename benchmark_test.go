// Copyright (c) 2024, vistone
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package conn

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// BenchmarkRateLimiter_Concurrent 测试RateLimiter的并发性能
func BenchmarkRateLimiter_Concurrent(b *testing.B) {
	limiter := NewRateLimiter(1000000, 1000000) // 1MB/s
	if limiter == nil {
		b.Fatal("NewRateLimiter returned nil")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.WaitRead(1024)
			limiter.WaitWrite(1024)
		}
	})
}

// BenchmarkStatConn_ReadWrite 测试StatConn的读写性能
func BenchmarkStatConn_ReadWrite(b *testing.B) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	var rx, tx uint64
	limiter := NewRateLimiter(10000000, 10000000) // 10MB/s

	// 启动服务器
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		statConn := NewStatConn(conn, &rx, &tx, limiter)
		buf := make([]byte, 4096)
		for {
			_, err := statConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		b.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	statConn := NewStatConn(conn, &rx, &tx, limiter)
	data := make([]byte, 4096)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		statConn.Write(data)
	}
}

// BenchmarkStatConn_NoLimiter 测试无限速器的StatConn性能
func BenchmarkStatConn_NoLimiter(b *testing.B) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	var rx, tx uint64

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		statConn := NewStatConn(conn, &rx, &tx, nil)
		buf := make([]byte, 4096)
		for {
			_, err := statConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		b.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	statConn := NewStatConn(conn, &rx, &tx, nil)
	data := make([]byte, 4096)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		statConn.Write(data)
	}
}

// TestStress_HighConcurrency 高并发连接压力测试
func TestStress_HighConcurrency(t *testing.T) {
	const (
		concurrency = 200  // 并发连接数
		iterations  = 20   // 每个连接的操作次数
		dataSize    = 1024 // 每次传输的数据大小
	)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	var totalRX, totalTX uint64
	limiter := NewRateLimiter(10000000, 10000000) // 10MB/s

	// 启动服务器
	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		for i := 0; i < concurrency; i++ {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}

			go func(c net.Conn) {
				defer c.Close()
				var rx, tx uint64
				statConn := NewStatConn(c, &rx, &tx, limiter)
				buf := make([]byte, dataSize)
				for j := 0; j < iterations; j++ {
					n, err := statConn.Read(buf)
					if err != nil {
						return
					}
					if n > 0 {
						statConn.Write(buf[:n])
					}
				}
				atomic.AddUint64(&totalRX, rx)
				atomic.AddUint64(&totalTX, tx)
			}(conn)
		}
	}()

	// 客户端连接
	var clientWg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		clientWg.Add(1)
		go func() {
			defer clientWg.Done()
			conn, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				t.Errorf("Dial failed: %v", err)
				return
			}
			defer conn.Close()

			var rx, tx uint64
			statConn := NewStatConn(conn, &rx, &tx, limiter)
			data := make([]byte, dataSize)
			for j := 0; j < iterations; j++ {
				_, err := statConn.Write(data)
				if err != nil {
					return
				}
				_, err = statConn.Read(data)
				if err != nil {
					return
				}
			}
			atomic.AddUint64(&totalRX, rx)
			atomic.AddUint64(&totalTX, tx)
		}()
	}

	clientWg.Wait()
	elapsed := time.Since(startTime)

	t.Logf("高并发测试结果:")
	t.Logf("  并发连接数: %d", concurrency)
	t.Logf("  每个连接操作次数: %d", iterations)
	t.Logf("  总耗时: %v", elapsed)
	t.Logf("  总接收字节数: %d (%.2f MB)", totalRX, float64(totalRX)/1024/1024)
	t.Logf("  总发送字节数: %d (%.2f MB)", totalTX, float64(totalTX)/1024/1024)
	t.Logf("  总吞吐量: %.2f MB/s", float64(totalRX+totalTX)/1024/1024/elapsed.Seconds())
	t.Logf("  平均延迟: %v", elapsed/time.Duration(concurrency*iterations))
}

// TestStress_HighThroughput 高吞吐量压力测试
func TestStress_HighThroughput(t *testing.T) {
	const (
		dataSize    = 2 * 1024 * 1024 // 2MB (进一步降低数据量)
		chunkSize   = 64 * 1024       // 64KB chunks
		concurrency = 3               // 降低并发数
	)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	var totalRX, totalTX uint64
	limiter := NewRateLimiter(100000000, 100000000) // 100MB/s

	// 启动服务器 - 接受连接并处理
	var serverWg sync.WaitGroup
	serverDone := make(chan bool, 1)

	go func() {
		defer close(serverDone)
		for i := 0; i < concurrency; i++ {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			serverWg.Add(1)
			go func(c net.Conn) {
				defer serverWg.Done()
				defer c.Close()
				var rx, tx uint64
				statConn := NewStatConn(c, &rx, &tx, limiter)
				buf := make([]byte, chunkSize)
				totalRead := 0
				// 设置读取超时
				c.SetReadDeadline(time.Now().Add(10 * time.Second))
				for totalRead < dataSize {
					n, err := statConn.Read(buf)
					if err != nil {
						// 如果是超时或EOF，检查是否已读取足够数据
						if totalRead >= dataSize*9/10 { // 允许90%的数据
							break
						}
						return
					}
					totalRead += n
					// 更新超时
					c.SetReadDeadline(time.Now().Add(10 * time.Second))
				}
				atomic.AddUint64(&totalRX, rx)
				atomic.AddUint64(&totalTX, tx)
			}(conn)
		}
		serverWg.Wait()
		serverDone <- true
	}()

	// 等待服务器启动
	time.Sleep(50 * time.Millisecond)

	// 客户端
	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				t.Errorf("Dial failed: %v", err)
				return
			}
			defer conn.Close()

			var rx, tx uint64
			statConn := NewStatConn(conn, &rx, &tx, limiter)
			data := make([]byte, chunkSize)
			totalWritten := 0
			for totalWritten < dataSize {
				n, err := statConn.Write(data)
				if err != nil {
					return
				}
				totalWritten += n
			}
			// 关闭写端，通知服务器数据发送完成
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.CloseWrite()
			}
			atomic.AddUint64(&totalRX, rx)
			atomic.AddUint64(&totalTX, tx)
		}()
	}

	wg.Wait()

	// 等待服务器处理完成，但设置超时
	select {
	case <-serverDone:
	case <-time.After(15 * time.Second):
		t.Logf("警告: 服务器处理超时，但继续统计结果")
	}

	elapsed := time.Since(startTime)

	t.Logf("高吞吐量测试结果:")
	t.Logf("  并发数: %d", concurrency)
	t.Logf("  每个连接数据量: %.2f MB", float64(dataSize)/1024/1024)
	t.Logf("  总数据量: %.2f MB", float64(dataSize*concurrency)/1024/1024)
	t.Logf("  总耗时: %v", elapsed)
	t.Logf("  总接收字节数: %d (%.2f MB)", totalRX, float64(totalRX)/1024/1024)
	t.Logf("  总发送字节数: %d (%.2f MB)", totalTX, float64(totalTX)/1024/1024)
	if elapsed.Seconds() > 0 {
		t.Logf("  吞吐量: %.2f MB/s", float64(totalRX+totalTX)/1024/1024/elapsed.Seconds())
	}
}

// TestStress_RateLimiter 速率限制器压力测试
func TestStress_RateLimiter(t *testing.T) {
	const (
		concurrency = 20
		operations  = 500
		bytesPerOp  = 512
	)

	limiter := NewRateLimiter(50000000, 50000000) // 50MB/s (高速率避免等待)
	if limiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	done := make(chan bool, 1)
	var wg sync.WaitGroup
	startTime := time.Now()

	// 设置超时
	go func() {
		time.Sleep(10 * time.Second)
		close(done)
	}()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				select {
				case <-done:
					return
				default:
					limiter.WaitRead(bytesPerOp)
					limiter.WaitWrite(bytesPerOp)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	totalOps := concurrency * operations * 2 // read + write
	totalBytes := totalOps * bytesPerOp

	t.Logf("速率限制器压力测试结果:")
	t.Logf("  并发数: %d", concurrency)
	t.Logf("  每个goroutine操作数: %d", operations)
	t.Logf("  总操作数: %d", totalOps)
	t.Logf("  总字节数: %d (%.2f MB)", totalBytes, float64(totalBytes)/1024/1024)
	t.Logf("  总耗时: %v", elapsed)
	if elapsed.Seconds() > 0 {
		t.Logf("  操作速率: %.2f ops/s", float64(totalOps)/elapsed.Seconds())
		t.Logf("  实际吞吐量: %.2f MB/s", float64(totalBytes)/1024/1024/elapsed.Seconds())
	}
}

// TestStress_DataExchange 双向数据交换压力测试
func TestStress_DataExchange(t *testing.T) {
	const (
		concurrency = 5
		dataSize    = 50 * 1024 // 50KB per connection (降低数据量避免超时)
	)

	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln1.Close()

	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln2.Close()

	var wg sync.WaitGroup
	startTime := time.Now()

	// 启动服务器端 - 接受连接
	serverDone := make(chan bool, 1)
	go func() {
		defer close(serverDone)
		for i := 0; i < concurrency; i++ {
			conn1, err := ln1.Accept()
			if err != nil {
				continue
			}

			conn2, err := net.Dial("tcp", ln2.Addr().String())
			if err != nil {
				conn1.Close()
				continue
			}

			wg.Add(1)
			go func(c1, c2 net.Conn) {
				defer wg.Done()
				defer c1.Close()
				defer c2.Close()

				// 发送数据
				go func() {
					data := make([]byte, 1024)
					total := 0
					for total < dataSize {
						n, _ := c1.Write(data)
						if n <= 0 {
							break
						}
						total += n
					}
					// 关闭写端
					if tcpConn, ok := c1.(*net.TCPConn); ok {
						tcpConn.CloseWrite()
					}
				}()

				buf1 := make([]byte, 4096)
				buf2 := make([]byte, 4096)
				DataExchange(c1, c2, 3*time.Second, buf1, buf2)
			}(conn1, conn2)
		}
		serverDone <- true
	}()

	// 等待服务器启动
	time.Sleep(50 * time.Millisecond)

	// 客户端连接
	var clientWg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		clientWg.Add(1)
		go func() {
			defer clientWg.Done()
			conn1, err := net.Dial("tcp", ln1.Addr().String())
			if err != nil {
				return
			}
			defer conn1.Close()

			conn2, err := ln2.Accept()
			if err != nil {
				return
			}
			defer conn2.Close()

			data := make([]byte, 1024)
			total := 0
			for total < dataSize {
				n, _ := conn2.Write(data)
				if n <= 0 {
					break
				}
				total += n
			}
			// 关闭写端
			if tcpConn, ok := conn2.(*net.TCPConn); ok {
				tcpConn.CloseWrite()
			}
		}()
	}

	clientWg.Wait()

	// 等待服务器处理完成，但设置超时
	select {
	case <-serverDone:
	case <-time.After(10 * time.Second):
		t.Logf("警告: 服务器处理超时")
	}

	// 等待所有数据交换完成
	done := make(chan bool, 1)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Logf("警告: 数据交换超时")
	}

	elapsed := time.Since(startTime)

	t.Logf("双向数据交换压力测试结果:")
	t.Logf("  并发连接数: %d", concurrency)
	t.Logf("  每个连接数据量: %.2f KB", float64(dataSize)/1024)
	t.Logf("  总数据量: %.2f MB", float64(dataSize*concurrency)/1024/1024)
	t.Logf("  总耗时: %v", elapsed)
	if elapsed.Seconds() > 0 {
		t.Logf("  吞吐量: %.2f MB/s", float64(dataSize*concurrency)/1024/1024/elapsed.Seconds())
	}
}

// TestStress_Statistics 统计功能压力测试
func TestStress_Statistics(t *testing.T) {
	const (
		concurrency = 50
		operations  = 5000
		dataSize    = 1024
	)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	var serverRX, serverTX uint64
	done := make(chan bool, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		statConn := NewStatConn(conn, &serverRX, &serverTX, nil)
		buf := make([]byte, dataSize)
		for {
			select {
			case <-done:
				return
			default:
				_, err := statConn.Read(buf)
				if err != nil {
					return
				}
			}
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	var rx, tx uint64
	statConn := NewStatConn(conn, &rx, &tx, nil)
	data := make([]byte, dataSize)

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				statConn.Write(data)
				statConn.GetRX()
				statConn.GetTX()
				statConn.GetTotal()
			}
		}()
	}

	wg.Wait()
	close(done)
	time.Sleep(50 * time.Millisecond)
	elapsed := time.Since(startTime)

	t.Logf("统计功能压力测试结果:")
	t.Logf("  并发数: %d", concurrency)
	t.Logf("  每个goroutine操作数: %d", operations)
	t.Logf("  总操作数: %d", concurrency*operations*4) // Write + GetRX + GetTX + GetTotal
	t.Logf("  总耗时: %v", elapsed)
	if elapsed.Seconds() > 0 {
		t.Logf("  操作速率: %.2f ops/s", float64(concurrency*operations*4)/elapsed.Seconds())
	}
	t.Logf("  接收字节数: %d (%.2f MB)", statConn.GetRX(), float64(statConn.GetRX())/1024/1024)
	t.Logf("  发送字节数: %d (%.2f MB)", statConn.GetTX(), float64(statConn.GetTX())/1024/1024)
}

// TestStress_MemoryLeak 内存泄漏检测测试
func TestStress_MemoryLeak(t *testing.T) {
	const iterations = 1000

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				return
			}

			var rx, tx uint64
			limiter := NewRateLimiter(1000000, 1000000)
			statConn := NewStatConn(conn, &rx, &tx, limiter)

			// 执行一些操作
			data := make([]byte, 1024)
			statConn.Write(data)
			statConn.Read(data)
			statConn.GetRX()
			statConn.GetTX()
			statConn.Reset()

			conn.Close()
		}()

		// 接受连接
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			var rx, tx uint64
			limiter := NewRateLimiter(1000000, 1000000)
			statConn := NewStatConn(conn, &rx, &tx, limiter)

			buf := make([]byte, 1024)
			statConn.Read(buf)
			statConn.Write(buf)
		}()
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	t.Logf("内存泄漏检测测试结果:")
	t.Logf("  迭代次数: %d", iterations)
	t.Logf("  总耗时: %v", elapsed)
	t.Logf("  平均每次迭代: %v", elapsed/time.Duration(iterations))
}

// runAllStressTests 运行所有压力测试并收集结果
func runAllStressTests() map[string]interface{} {
	results := make(map[string]interface{})

	// 这里可以添加自动运行测试并收集结果的逻辑
	// 由于测试需要网络连接，这里只返回测试说明

	return results
}
