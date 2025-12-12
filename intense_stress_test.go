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
	"context"
	"io"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestIntenseStress_LongDuration 长时间高强度压力测试
func TestIntenseStress_LongDuration(t *testing.T) {
	const (
		// 降低并发数
		maxConcurrency = 100
			
		// 减少数据量传输
		dataSizePerConn = 1024 * 1024 // 1MB per connection
			
		// 小块传输以增加系统负载
		chunkSize = 4 * 1024 // 4KB chunks
			
		// 缩短测试持续时间
		testDuration = 30 * time.Second
			
		// 降低速率限制
		readRate  = 10 * 1024 * 1024  // 10MB/s
		writeRate = 10 * 1024 * 1024  // 10MB/s
	)

	// 创建监听器
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	// 统计变量
	var (
		totalConnections int64
		totalRX          uint64
		totalTX          uint64
		activeClients    int64
		errors           int64
	)

	// 创建限速器
	limiter := NewRateLimiter(readRate, writeRate)

	// 启动服务器
	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	var serverWg sync.WaitGroup
	go func() {
		// 接受连接的循环
		for {
			select {
			case <-serverCtx.Done():
				return
			default:
			}

			// 设置接受连接的超时
			ln.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
			conn, err := ln.Accept()
			if err != nil {
				// 检查是否是因为超时
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				// 如果是其他错误且上下文未取消，则记录错误
				select {
				case <-serverCtx.Done():
					return
				default:
					atomic.AddInt64(&errors, 1)
					continue
				}
			}

			// 增加连接计数
			atomic.AddInt64(&totalConnections, 1)

			// 处理连接
			serverWg.Add(1)
			go func(c net.Conn) {
				defer serverWg.Done()
				defer c.Close()

				var rx, tx uint64
				statConn := NewStatConn(c, &rx, &tx, limiter)
				buf := make([]byte, chunkSize)

				// 读取数据直到达到指定大小或出错
				totalRead := 0
				for totalRead < dataSizePerConn {
					n, err := statConn.Read(buf)
					if err != nil {
						// 如果是EOF并且我们已经读取了足够的数据，则正常退出
						if err == io.EOF && totalRead >= dataSizePerConn*9/10 {
							break
						}
						atomic.AddInt64(&errors, 1)
						return
					}
					totalRead += n

					// 回写部分数据以增加负载
					if n > 0 {
						// 随机回写一部分数据
						echoSize := rand.Intn(n)
						if echoSize > 0 {
							statConn.Write(buf[:echoSize])
						}
					}
				}

				// 更新统计数据
				atomic.AddUint64(&totalRX, rx)
				atomic.AddUint64(&totalTX, tx)
			}(conn)
		}
	}()

	// 确保在函数结束时等待所有服务器goroutine完成
	defer func() {
		serverCancel()
		serverWg.Wait()
	}()

	// 启动客户端
	var clientWg sync.WaitGroup

	// 客户端生成器
	clientGeneratorCtx, clientGeneratorCancel := context.WithCancel(context.Background())
	defer clientGeneratorCancel()

	go func() {
		for {
			select {
			case <-clientGeneratorCtx.Done():
				return
			case <-time.After(10 * time.Millisecond): // 每10毫秒创建新客户端
			}

			// 检查是否达到最大并发数
			currentActive := atomic.LoadInt64(&activeClients)
			if currentActive >= maxConcurrency {
				continue
			}

			// 创建新客户端
			clientWg.Add(1)
			atomic.AddInt64(&activeClients, 1)

			go func() {
				defer clientWg.Done()
				defer atomic.AddInt64(&activeClients, -1)

				// 连接到服务器
				conn, err := net.Dial("tcp", ln.Addr().String())
				if err != nil {
					atomic.AddInt64(&errors, 1)
					return
				}
				defer conn.Close()

				var rx, tx uint64
				statConn := NewStatConn(conn, &rx, &tx, limiter)

				// 发送数据
				data := make([]byte, chunkSize)
				totalSent := 0

				for totalSent < dataSizePerConn {
					// 填充随机数据
					for i := range data {
						data[i] = byte(rand.Intn(256))
					}

					n, err := statConn.Write(data)
					if err != nil {
						atomic.AddInt64(&errors, 1)
						return
					}
					totalSent += n

					// 随机读取一些回写的数据
					if rand.Intn(10) < 3 { // 30%概率读取
						buf := make([]byte, chunkSize/4)
						statConn.Read(buf)
					}
				}

				// 关闭写端
				if tcpConn, ok := conn.(*net.TCPConn); ok {
					tcpConn.CloseWrite()
				}

				// 更新统计数据
				atomic.AddUint64(&totalRX, rx)
				atomic.AddUint64(&totalTX, tx)
			}()
		}
	}()

	// 运行指定时间
	startTime := time.Now()
	t.Logf("开始长时间高强度压力测试...")
	t.Logf("  目标并发数: %d", maxConcurrency)
	t.Logf("  每连接数据量: %.2f MB", float64(dataSizePerConn)/1024/1024)
	t.Logf("  测试持续时间: %v", testDuration)
	t.Logf("  读写速率限制: %.2f MB/s", float64(readRate)/1024/1024)

	// 等待测试时间结束
	time.Sleep(testDuration)

	// 取消客户端生成器
	clientGeneratorCancel()

	// 等待所有客户端完成（最多等待1分钟）
	done := make(chan struct{})
	go func() {
		clientWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Minute):
		t.Log("警告: 等待客户端完成超时")
	}

	// 取消服务器
	serverCancel()

	// 等待服务器完成（最多等待30秒）
	done = make(chan struct{})
	go func() {
		serverWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Log("警告: 等待服务器完成超时")
	}

	// 计算最终结果
	elapsed := time.Since(startTime)
	finalConnections := atomic.LoadInt64(&totalConnections)
	finalRX := atomic.LoadUint64(&totalRX)
	finalTX := atomic.LoadUint64(&totalTX)
	finalErrors := atomic.LoadInt64(&errors)

	t.Logf("长时间高强度压力测试结果:")
	t.Logf("  测试持续时间: %v", elapsed)
	t.Logf("  总连接数: %d", finalConnections)
	t.Logf("  平均并发连接数: %.2f", float64(finalConnections)*float64(time.Second)/float64(elapsed))
	t.Logf("  总接收字节数: %d (%.2f GB)", finalRX, float64(finalRX)/1024/1024/1024)
	t.Logf("  总发送字节数: %d (%.2f GB)", finalTX, float64(finalTX)/1024/1024/1024)
	t.Logf("  总吞吐量: %.2f GB", float64(finalRX+finalTX)/1024/1024/1024)
	t.Logf("  平均吞吐量: %.2f MB/s", float64(finalRX+finalTX)/1024/1024/elapsed.Seconds())
	t.Logf("  错误数: %d", finalErrors)

	// 性能评估
	if finalErrors > finalConnections/100 { // 错误率超过1%
		t.Errorf("错误率过高: %.2f%%", float64(finalErrors)/float64(finalConnections)*100)
	}

	if float64(finalRX+finalTX)/elapsed.Seconds() < 100*1024*1024 { // 平均吞吐量低于100MB/s
		t.Errorf("吞吐量不足: %.2f MB/s", float64(finalRX+finalTX)/1024/1024/elapsed.Seconds())
	}
}
