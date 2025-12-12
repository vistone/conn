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
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// minInt 返回两个整数中的较小值
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestIntenseStress_Statistics 极限统计功能压力测试
func TestIntenseStress_Statistics(t *testing.T) {
	const (
		// 降低并发数以减少资源消耗
		concurrency = 100

		// 减少操作数
		operationsPerGoroutine = 1000

		// 小数据块
		dataSize = 256

		// 缩短测试持续时间
		testDuration = 30 * time.Second
	)

	// 创建监听器
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	// 全局统计变量
	var (
		globalRX uint64
		globalTX uint64
	)

	// 创建多个独立的统计组
	type statGroup struct {
		rx, tx uint64
	}

	statGroups := make([]*statGroup, 100)
	for i := range statGroups {
		statGroups[i] = &statGroup{}
	}

	// 统计操作计数器
	var (
		totalOps     int64
		completedOps int64
		errors       int64
	)

	// 启动服务器
	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	var serverWg sync.WaitGroup
	go func() {
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

			// 处理连接
			serverWg.Add(1)
			go func(c net.Conn) {
				defer serverWg.Done()
				defer c.Close()

				// 随机选择一个统计组
				groupIdx := rand.Intn(len(statGroups))
				stat := statGroups[groupIdx]

				var rx, tx uint64
				statConn := NewStatConn(c, &rx, &tx, nil)
				buf := make([]byte, dataSize)

				// 执行随机操作
				for i := 0; i < 100; i++ {
					// 随机选择操作类型
					switch rand.Intn(4) { // 改为4种操作类型
					case 0: // 写操作（写入随机数据）
						// 生成随机数据
						for j := range buf {
							buf[j] = byte(rand.Intn(256))
						}
						_, err := statConn.Write(buf)
						if err != nil {
							atomic.AddInt64(&errors, 1)
							return
						}
						atomic.AddInt64(&completedOps, 1)
					case 1: // 统计查询操作
						statConn.GetRX()
						statConn.GetTX()
						statConn.GetTotal()
						atomic.AddInt64(&completedOps, 1)
					case 2: // 重置操作
						statConn.Reset()
						atomic.AddInt64(&completedOps, 1)
					case 3: // 简单读操作（尝试读取少量数据）
						readBuf := make([]byte, 16) // 只读取少量数据
						statConn.Read(readBuf)
						atomic.AddInt64(&completedOps, 1)
					}
				}

				// 更新统计数据
				atomic.AddUint64(&stat.rx, rx)
				atomic.AddUint64(&stat.tx, tx)
				atomic.AddUint64(&globalRX, rx)
				atomic.AddUint64(&globalTX, tx)
			}(conn)
		}
	}()

	// 确保在函数结束时等待所有服务器goroutine完成
	defer func() {
		serverCancel()
		serverWg.Wait()
	}()

	// 创建上下文用于控制测试时间
	testCtx, testCancel := context.WithTimeout(context.Background(), testDuration)
	defer testCancel()

	// 设置总体超时，防止测试hang住
	overallTimeout, overallCancel := context.WithTimeout(context.Background(), testDuration+2*time.Minute)
	defer overallCancel()

	// 启动压力测试协程
	var clientWg sync.WaitGroup

	// 记录开始时间
	startTime := time.Now()
	t.Logf("开始极限统计功能压力测试...")
	t.Logf("  并发数: %d", concurrency)
	t.Logf("  每协程操作数: %d", operationsPerGoroutine)
	t.Logf("  每操作数据大小: %d 字节", dataSize)
	t.Logf("  统计组数量: %d", len(statGroups))
	t.Logf("  测试持续时间: %v", testDuration)

	for i := 0; i < concurrency; i++ {
		clientWg.Add(1)
		atomic.AddInt64(&totalOps, operationsPerGoroutine)

		go func(goroutineID int) {
			defer clientWg.Done()

			// 连接到服务器
			conn, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				atomic.AddInt64(&errors, 1)
				return
			}
			defer conn.Close()

			// 随机选择一个统计组
			groupIdx := rand.Intn(len(statGroups))
			stat := statGroups[groupIdx]

			var rx, tx uint64
			statConn := NewStatConn(conn, &rx, &tx, nil)
			data := make([]byte, dataSize)

			for j := 0; j < operationsPerGoroutine; j++ {
				// 检查是否超时
				select {
				case <-testCtx.Done():
					// 更新未完成的操作数
					remainingOps := operationsPerGoroutine - j
					atomic.AddInt64(&totalOps, -int64(remainingOps))
					return
				case <-overallTimeout.Done():
					// 更新未完成的操作数
					remainingOps := operationsPerGoroutine - j
					atomic.AddInt64(&totalOps, -int64(remainingOps))
					return
				default:
				}

				// 随机选择操作类型
				switch rand.Intn(4) {
				case 0: // 写操作
					_, err := statConn.Write(data)
					if err != nil {
						atomic.AddInt64(&errors, 1)
					} else {
						atomic.AddInt64(&completedOps, 1)
					}
				case 1: // 统计查询操作
					statConn.GetRX()
					statConn.GetTX()
					statConn.GetTotal()
					atomic.AddInt64(&completedOps, 1)
				case 2: // 重置操作
					statConn.Reset()
					atomic.AddInt64(&completedOps, 1)
				case 3: // 读操作
					readBuf := make([]byte, minInt(dataSize, 16)) // 只读取少量数据
					statConn.Read(readBuf)
					atomic.AddInt64(&completedOps, 1)
				}
			}

			// 更新统计数据
			atomic.AddUint64(&stat.rx, rx)
			atomic.AddUint64(&stat.tx, tx)
			atomic.AddUint64(&globalRX, rx)
			atomic.AddUint64(&globalTX, tx)
		}(i)
	}

	// 启动统计监控协程
	var monitorWg sync.WaitGroup
	monitorWg.Add(1)
	go func() {
		defer monitorWg.Done()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-testCtx.Done():
				return
			case <-overallTimeout.Done():
				return
			case <-ticker.C:
				currentTotal := atomic.LoadInt64(&totalOps)
				currentCompleted := atomic.LoadInt64(&completedOps)
				currentErrors := atomic.LoadInt64(&errors)
				currentGlobalRX := atomic.LoadUint64(&globalRX)
				currentGlobalTX := atomic.LoadUint64(&globalTX)

				t.Logf("统计功能监控报告:")
				t.Logf("  总操作数: %d", currentTotal)
				t.Logf("  已完成操作数: %d", currentCompleted)
				t.Logf("  错误数: %d", currentErrors)
				t.Logf("  全局接收字节数: %d (%.2f MB)", currentGlobalRX, float64(currentGlobalRX)/1024/1024)
				t.Logf("  全局发送字节数: %d (%.2f MB)", currentGlobalTX, float64(currentGlobalTX)/1024/1024)
				if currentTotal > 0 {
					t.Logf("  完成率: %.2f%%", float64(currentCompleted)/float64(currentTotal)*100)
					t.Logf("  错误率: %.2f%%", float64(currentErrors)/float64(currentTotal)*100)
				}
			}
		}
	}()

	// 等待所有客户端协程完成或超时
	done := make(chan struct{})
	go func() {
		clientWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("所有客户端协程已完成")
	case <-testCtx.Done():
		t.Log("测试时间已到，正在等待协程完成...")
		// 给协程一些时间来完成清理工作
		time.Sleep(10 * time.Second)
	case <-overallTimeout.Done():
		t.Log("总体超时，强制终止测试...")
	}

	// 取消服务器和监控协程
	serverCancel()
	testCancel()

	// 等待服务器和监控协程完成
	serverWg.Wait()
	monitorWg.Wait()

	// 计算最终结果
	elapsed := time.Since(startTime)
	finalTotalOps := atomic.LoadInt64(&totalOps)
	finalCompletedOps := atomic.LoadInt64(&completedOps)
	finalErrors := atomic.LoadInt64(&errors)
	finalGlobalRX := atomic.LoadUint64(&globalRX)
	finalGlobalTX := atomic.LoadUint64(&globalTX)

	t.Logf("极限统计功能压力测试结果:")
	t.Logf("  测试持续时间: %v", elapsed)
	t.Logf("  总操作数: %d", finalTotalOps)
	t.Logf("  已完成操作数: %d", finalCompletedOps)
	t.Logf("  错误数: %d", finalErrors)
	t.Logf("  全局接收字节数: %d (%.2f GB)", finalGlobalRX, float64(finalGlobalRX)/1024/1024/1024)
	t.Logf("  全局发送字节数: %d (%.2f GB)", finalGlobalTX, float64(finalGlobalTX)/1024/1024/1024)
	if elapsed.Seconds() > 0 {
		t.Logf("  操作速率: %.2f ops/s", float64(finalCompletedOps)/elapsed.Seconds())
		t.Logf("  实际吞吐量: %.2f MB/s", float64(finalGlobalRX+finalGlobalTX)/1024/1024/elapsed.Seconds())
	}
	if finalTotalOps > 0 {
		t.Logf("  完成率: %.2f%%", float64(finalCompletedOps)/float64(finalTotalOps)*100)
		t.Logf("  错误率: %.2f%%", float64(finalErrors)/float64(finalTotalOps)*100)
	}

	// 显示各个统计组的信息
	t.Logf("各统计组详情:")
	for i, group := range statGroups {
		if i >= 10 { // 只显示前10个组以避免日志过长
			t.Logf("  ... (还有%d个统计组)", len(statGroups)-10)
			break
		}
		groupRX := atomic.LoadUint64(&group.rx)
		groupTX := atomic.LoadUint64(&group.tx)
		if groupRX > 0 || groupTX > 0 {
			t.Logf("  组 #%d: RX=%d bytes (%.2f MB), TX=%d bytes (%.2f MB)",
				i, groupRX, float64(groupRX)/1024/1024,
				groupTX, float64(groupTX)/1024/1024)
		}
	}

	// 性能评估
	if float64(finalErrors)/float64(finalTotalOps) > 0.001 { // 错误率超过0.1%
		t.Errorf("错误率过高: %.2f%%", float64(finalErrors)/float64(finalTotalOps)*100)
	}

	// 验证统计数据的一致性
	totalGroupRX := uint64(0)
	totalGroupTX := uint64(0)
	for _, group := range statGroups {
		totalGroupRX += atomic.LoadUint64(&group.rx)
		totalGroupTX += atomic.LoadUint64(&group.tx)
	}

	if totalGroupRX != finalGlobalRX || totalGroupTX != finalGlobalTX {
		t.Errorf("统计数据不一致: 组总计(RX=%d, TX=%d) != 全局总计(RX=%d, TX=%d)",
			totalGroupRX, totalGroupTX, finalGlobalRX, finalGlobalTX)
	}

	t.Log("极限统计功能压力测试完成")
}
