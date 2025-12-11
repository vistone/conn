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
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// TestRateLimiter_Basic 测试RateLimiter的基本功能
func TestRateLimiter_Basic(t *testing.T) {
	// 测试创建限速器
	limiter := NewRateLimiter(1000, 2000)
	if limiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	// 测试等待读取令牌
	start := time.Now()
	limiter.WaitRead(100)
	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Errorf("WaitRead took too long: %v", elapsed)
	}

	// 测试等待写入令牌
	start = time.Now()
	limiter.WaitWrite(200)
	elapsed = time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Errorf("WaitWrite took too long: %v", elapsed)
	}
}

// TestRateLimiter_Nil 测试nil限速器
func TestRateLimiter_Nil(t *testing.T) {
	var limiter *RateLimiter
	limiter.WaitRead(100)
	limiter.WaitWrite(100)
	limiter.SetRate(1000, 2000)
	limiter.Reset()
	// 不应该panic
}

// TestRateLimiter_ZeroRate 测试零速率
func TestRateLimiter_ZeroRate(t *testing.T) {
	// 两个速率都为0，应该返回nil
	limiter := NewRateLimiter(0, 0)
	if limiter != nil {
		t.Error("NewRateLimiter(0, 0) should return nil")
	}

	// 一个速率为0，应该设置为无限制
	limiter = NewRateLimiter(0, 1000)
	if limiter == nil {
		t.Fatal("NewRateLimiter(0, 1000) should not return nil")
	}

	limiter = NewRateLimiter(1000, 0)
	if limiter == nil {
		t.Fatal("NewRateLimiter(1000, 0) should not return nil")
	}
}

// TestRateLimiter_SetRate 测试动态调整速率
func TestRateLimiter_SetRate(t *testing.T) {
	limiter := NewRateLimiter(1000, 2000)
	if limiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	limiter.SetRate(1500, 3000)
	// 不应该panic
}

// TestRateLimiter_Reset 测试重置限速器
func TestRateLimiter_Reset(t *testing.T) {
	limiter := NewRateLimiter(1000, 2000)
	if limiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	limiter.Reset()
	// 不应该panic
}

// TestRateLimiter_Concurrent 测试并发访问
func TestRateLimiter_Concurrent(t *testing.T) {
	limiter := NewRateLimiter(10000, 10000)
	if limiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				limiter.WaitRead(10)
				limiter.WaitWrite(10)
			}
		}()
	}
	wg.Wait()
}

// TestStatConn_Basic 测试StatConn的基本功能
func TestStatConn_Basic(t *testing.T) {
	// 创建TCP连接
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	var rx, tx uint64
	limiter := NewRateLimiter(10000, 10000)

	// 启动服务器
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		statConn := NewStatConn(conn, &rx, &tx, limiter)
		buf := make([]byte, 1024)
		n, err := statConn.Read(buf)
		if err != nil && err != io.EOF {
			t.Errorf("Read failed: %v", err)
		}
		if n > 0 {
			statConn.Write(buf[:n])
		}
	}()

	// 客户端连接
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	statConn := NewStatConn(conn, &rx, &tx, limiter)
	testData := []byte("test data")
	n, err := statConn.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write returned %d, expected %d", n, len(testData))
	}

	buf := make([]byte, 1024)
	n, err = statConn.Read(buf)
	if err != nil && err != io.EOF {
		t.Errorf("Read failed: %v", err)
	}

	// 检查统计数据
	if statConn.GetRX() == 0 && statConn.GetTX() == 0 {
		t.Error("Statistics should not be zero")
	}
}

// TestStatConn_NilPointers 测试nil指针
func TestStatConn_NilPointers(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	// 测试nil指针
	statConn := NewStatConn(conn, nil, nil, nil)
	testData := []byte("test")
	_, err = statConn.Write(testData)
	if err != nil {
		t.Errorf("Write with nil pointers failed: %v", err)
	}

	// 测试GetRX和GetTX在nil指针时返回0
	if statConn.GetRX() != 0 {
		t.Error("GetRX should return 0 when RX is nil")
	}
	if statConn.GetTX() != 0 {
		t.Error("GetTX should return 0 when TX is nil")
	}

	statConn.Reset() // 不应该panic
}

// TestStatConn_TCPOperations 测试TCP特定操作
func TestStatConn_TCPOperations(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	var rx, tx uint64
	statConn := NewStatConn(conn, &rx, &tx, nil)

	// 测试TCP操作
	if !statConn.IsTCP() {
		t.Error("IsTCP should return true for TCP connection")
	}

	if statConn.IsUDP() {
		t.Error("IsUDP should return false for TCP connection")
	}

	if statConn.NetworkType() != "tcp" {
		t.Errorf("NetworkType should return 'tcp', got '%s'", statConn.NetworkType())
	}

	// 测试TCP特定方法
	err = statConn.SetKeepAlive(true)
	if err != nil {
		t.Errorf("SetKeepAlive failed: %v", err)
	}

	err = statConn.SetNoDelay(true)
	if err != nil {
		t.Errorf("SetNoDelay failed: %v", err)
	}

	err = statConn.SetLinger(0)
	if err != nil {
		t.Errorf("SetLinger failed: %v", err)
	}
}

// TestStatConn_UDPOperations 测试UDP特定操作
func TestStatConn_UDPOperations(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ResolveUDPAddr failed: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("ListenUDP failed: %v", err)
	}
	defer conn.Close()

	var rx, tx uint64
	statConn := NewStatConn(conn, &rx, &tx, nil)

	// 测试UDP操作
	if !statConn.IsUDP() {
		t.Error("IsUDP should return true for UDP connection")
	}

	if statConn.IsTCP() {
		t.Error("IsTCP should return false for UDP connection")
	}

	if statConn.NetworkType() != "udp" {
		t.Errorf("NetworkType should return 'udp', got '%s'", statConn.NetworkType())
	}

	// 测试UDP特定方法
	err = statConn.SetReadBuffer(4096)
	if err != nil {
		t.Errorf("SetReadBuffer failed: %v", err)
	}

	err = statConn.SetWriteBuffer(4096)
	if err != nil {
		t.Errorf("SetWriteBuffer failed: %v", err)
	}
}

// TestStatConn_Reset 测试重置统计数据
func TestStatConn_Reset(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	var rx, tx uint64
	statConn := NewStatConn(conn, &rx, &tx, nil)

	testData := []byte("test")
	statConn.Write(testData)

	if statConn.GetTX() == 0 {
		t.Error("GetTX should not be zero after write")
	}

	statConn.Reset()

	if statConn.GetRX() != 0 || statConn.GetTX() != 0 {
		t.Error("Reset should clear statistics")
	}
}

// TestTimeoutReader 测试TimeoutReader
func TestTimeoutReader(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(100 * time.Millisecond)
		conn.Write([]byte("test"))
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	reader := &TimeoutReader{
		Conn:    conn,
		Timeout: 1 * time.Second,
	}

	buf := make([]byte, 1024)
	_, err = reader.Read(buf)
	if err != nil && err != io.EOF {
		t.Errorf("Read failed: %v", err)
	}
}

// TestDataExchange 测试DataExchange
func TestDataExchange(t *testing.T) {
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

	done := make(chan bool, 1)

	go func() {
		conn1, err := ln1.Accept()
		if err != nil {
			return
		}
		defer conn1.Close()

		conn2, err := net.Dial("tcp", ln2.Addr().String())
		if err != nil {
			return
		}
		defer conn2.Close()

		buf1 := make([]byte, 4096)
		buf2 := make([]byte, 4096)
		DataExchange(conn1, conn2, 100*time.Millisecond, buf1, buf2)
		done <- true
	}()

	conn1, err := net.Dial("tcp", ln1.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn1.Close()

	conn2, err := ln2.Accept()
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	defer conn2.Close()

	// 发送一些数据
	conn1.Write([]byte("test"))
	time.Sleep(50 * time.Millisecond)

	// 等待完成或超时
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("DataExchange timed out")
	}
}

// TestDataExchange_NilConn 测试nil连接
func TestDataExchange_NilConn(t *testing.T) {
	err := DataExchange(nil, nil, 0, nil, nil)
	if err == nil {
		t.Error("DataExchange should return error for nil connections")
	}
}

// TestStatConn_GetConn 测试GetConn
func TestStatConn_GetConn(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	var rx, tx uint64
	statConn := NewStatConn(conn, &rx, &tx, nil)

	if statConn.GetConn() != conn {
		t.Error("GetConn should return the underlying connection")
	}
}

// TestStatConn_GetRate 测试GetRate
func TestStatConn_GetRate(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	var rx, tx uint64
	limiter := NewRateLimiter(1000, 2000)
	statConn := NewStatConn(conn, &rx, &tx, limiter)

	if statConn.GetRate() != limiter {
		t.Error("GetRate should return the rate limiter")
	}
}

// TestStatConn_AsTCPConn 测试AsTCPConn
func TestStatConn_AsTCPConn(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	var rx, tx uint64
	statConn := NewStatConn(conn, &rx, &tx, nil)

	tcpConn, ok := statConn.AsTCPConn()
	if !ok {
		t.Error("AsTCPConn should return true for TCP connection")
	}
	if tcpConn == nil {
		t.Error("AsTCPConn should return non-nil TCP connection")
	}
}

// TestStatConn_AsUDPConn 测试AsUDPConn
func TestStatConn_AsUDPConn(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ResolveUDPAddr failed: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("ListenUDP failed: %v", err)
	}
	defer conn.Close()

	var rx, tx uint64
	statConn := NewStatConn(conn, &rx, &tx, nil)

	udpConn, ok := statConn.AsUDPConn()
	if !ok {
		t.Error("AsUDPConn should return true for UDP connection")
	}
	if udpConn == nil {
		t.Error("AsUDPConn should return non-nil UDP connection")
	}
}

// TestStatConn_TCPErrorOperations 测试TCP错误操作
func TestStatConn_TCPErrorOperations(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ResolveUDPAddr failed: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("ListenUDP failed: %v", err)
	}
	defer conn.Close()

	var rx, tx uint64
	statConn := NewStatConn(conn, &rx, &tx, nil)

	// UDP连接上调用TCP方法应该返回错误
	err = statConn.SetKeepAlive(true)
	if err == nil {
		t.Error("SetKeepAlive should return error for UDP connection")
	}

	err = statConn.CloseRead()
	if err == nil {
		t.Error("CloseRead should return error for UDP connection")
	}

	err = statConn.CloseWrite()
	if err == nil {
		t.Error("CloseWrite should return error for UDP connection")
	}
}

// TestStatConn_UDPErrorOperations 测试UDP错误操作
func TestStatConn_UDPErrorOperations(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	var rx, tx uint64
	statConn := NewStatConn(conn, &rx, &tx, nil)

	// TCP连接上调用UDP方法应该返回错误
	_, _, err = statConn.ReadFromUDP(make([]byte, 1024))
	if err == nil {
		t.Error("ReadFromUDP should return error for TCP connection")
	}

	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	_, err = statConn.WriteToUDP([]byte("test"), addr)
	if err == nil {
		t.Error("WriteToUDP should return error for TCP connection")
	}
}

// TestStatConn_GetTotal 测试GetTotal
func TestStatConn_GetTotal(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	var rx, tx uint64
	statConn := NewStatConn(conn, &rx, &tx, nil)

	testData := []byte("test")
	statConn.Write(testData)

	total := statConn.GetTotal()
	if total == 0 {
		t.Error("GetTotal should not be zero after write")
	}
	if total != statConn.GetRX()+statConn.GetTX() {
		t.Error("GetTotal should equal GetRX + GetTX")
	}
}
