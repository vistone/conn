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
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestIntenseStress_RateLimiter 极限速率限制器压力测试
func TestIntenseStress_RateLimiter(t *testing.T) {
	const (
		// 极高并发数
		concurrency = 1000

		// 超大操作数
		operationsPerGoroutine = 10000

		// 小字节操作以增加频率
		bytesPerOp = 64

		// 极高速率
		readRate  = 1000 * 1024 * 1024 // 1GB/s
		writeRate = 1000 * 1024 * 1024 // 1GB/s

		// 测试持续时间
		testDuration = 3 * time.Minute
	)

	// 创建限速器
	limiter := NewRateLimiter(readRate, writeRate)
	if limiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	// 统计变量
	var (
		totalOps     int64
		completedOps int64
		errors       int64
	)

	// 创建上下文用于控制测试时间
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	// 启动监控协程
	var monitorWg sync.WaitGroup
	monitorWg.Add(1)
	go func() {
		defer monitorWg.Done()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentTotal := atomic.LoadInt64(&totalOps)
				currentCompleted := atomic.LoadInt64(&completedOps)
				currentErrors := atomic.LoadInt64(&errors)

				t.Logf("速率限制器监控报告:")
				t.Logf("  总操作数: %d", currentTotal)
				t.Logf("  已完成操作数: %d", currentCompleted)
				t.Logf("  错误数: %d", currentErrors)
				if currentTotal > 0 {
					t.Logf("  完成率: %.2f%%", float64(currentCompleted)/float64(currentTotal)*100)
					t.Logf("  错误率: %.2f%%", float64(currentErrors)/float64(currentTotal)*100)
				}
			}
		}
	}()

	// 启动压力测试协程
	var wg sync.WaitGroup

	// 记录开始时间
	startTime := time.Now()
	t.Logf("开始极限速率限制器压力测试...")
	t.Logf("  并发数: %d", concurrency)
	t.Logf("  每协程操作数: %d", operationsPerGoroutine)
	t.Logf("  每操作字节数: %d", bytesPerOp)
	t.Logf("  读速率限制: %.2f GB/s", float64(readRate)/1024/1024/1024)
	t.Logf("  写速率限制: %.2f GB/s", float64(writeRate)/1024/1024/1024)
	t.Logf("  测试持续时间: %v", testDuration)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		atomic.AddInt64(&totalOps, operationsPerGoroutine*2) // read + write

		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				// 检查是否超时
				select {
				case <-ctx.Done():
					// 更新未完成的操作数
					remainingOps := operationsPerGoroutine - j
					atomic.AddInt64(&totalOps, -int64(remainingOps*2))
					return
				default:
				}

				// 执行读操作
				func() {
					defer func() {
						if r := recover(); r != nil {
							atomic.AddInt64(&errors, 1)
						}
					}()

					limiter.WaitRead(bytesPerOp)
					atomic.AddInt64(&completedOps, 1)
				}()

				// 执行写操作
				func() {
					defer func() {
						if r := recover(); r != nil {
							atomic.AddInt64(&errors, 1)
						}
					}()

					limiter.WaitWrite(bytesPerOp)
					atomic.AddInt64(&completedOps, 1)
				}()
			}
		}(i)
	}

	// 等待所有协程完成或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("所有协程已完成")
	case <-ctx.Done():
		t.Log("测试时间已到，正在等待协程完成...")
		// 给协程一些时间来完成清理工作
		time.Sleep(5 * time.Second)
	}

	// 取消监控协程
	cancel()
	monitorWg.Wait()

	// 计算最终结果
	elapsed := time.Since(startTime)
	finalTotalOps := atomic.LoadInt64(&totalOps)
	finalCompletedOps := atomic.LoadInt64(&completedOps)
	finalErrors := atomic.LoadInt64(&errors)

	t.Logf("极限速率限制器压力测试结果:")
	t.Logf("  测试持续时间: %v", elapsed)
	t.Logf("  总操作数: %d", finalTotalOps)
	t.Logf("  已完成操作数: %d", finalCompletedOps)
	t.Logf("  错误数: %d", finalErrors)
	if elapsed.Seconds() > 0 {
		t.Logf("  操作速率: %.2f ops/s", float64(finalCompletedOps)/elapsed.Seconds())
		t.Logf("  实际吞吐量: %.2f GB/s", float64(finalCompletedOps*bytesPerOp)/1024/1024/1024/elapsed.Seconds())
	}
	if finalTotalOps > 0 {
		t.Logf("  完成率: %.2f%%", float64(finalCompletedOps)/float64(finalTotalOps)*100)
		t.Logf("  错误率: %.2f%%", float64(finalErrors)/float64(finalTotalOps)*100)
	}

	// 性能评估
	if float64(finalErrors)/float64(finalTotalOps) > 0.001 { // 错误率超过0.1%
		t.Errorf("错误率过高: %.2f%%", float64(finalErrors)/float64(finalTotalOps)*100)
	}

	// 测试动态调整速率
	t.Log("测试动态调整速率...")

	// 调整为更低的速率
	lowerReadRate := int64(readRate / 10)   // 100MB/s
	lowerWriteRate := int64(writeRate / 10) // 100MB/s
	limiter.SetRate(lowerReadRate, lowerWriteRate)

	// 再次执行一些操作来验证速率调整
	testOps := 1000
	start := time.Now()
	for i := 0; i < testOps; i++ {
		limiter.WaitRead(bytesPerOp)
		limiter.WaitWrite(bytesPerOp)
	}
	adjustElapsed := time.Since(start)

	t.Logf("速率调整后测试结果:")
	t.Logf("  操作数: %d", testOps*2)
	t.Logf("  耗时: %v", adjustElapsed)
	if adjustElapsed.Seconds() > 0 {
		t.Logf("  实际吞吐量: %.2f MB/s", float64(testOps*2*bytesPerOp)/1024/1024/adjustElapsed.Seconds())
	}

	// 验证速率确实降低了（应该比原来慢至少5倍）
	originalExpectedTime := time.Duration(float64(testOps*2*bytesPerOp) * float64(time.Second) / float64(readRate))
	if adjustElapsed < originalExpectedTime*5 {
		t.Errorf("速率调整可能未生效，调整后耗时: %v, 预期至少: %v", adjustElapsed, originalExpectedTime*5)
	}

	t.Log("极限速率限制器压力测试完成")
}
