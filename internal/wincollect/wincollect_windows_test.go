//go:build windows

package wincollect

import (
	"net"
	"os"
	"testing"
	"time"
)

func TestConnectionsFindsOwnedLoopbackTCP(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
	}()

	client, err := net.Dial("tcp4", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := <-accepted
	defer server.Close()

	got, err := New().Connections()
	if err != nil {
		t.Fatal(err)
	}
	local := client.LocalAddr().(*net.TCPAddr).AddrPort()
	remote := client.RemoteAddr().(*net.TCPAddr).AddrPort()
	for _, conn := range got {
		if conn.Local == local && conn.Remote == remote {
			if conn.Process.PID != os.Getpid() {
				t.Fatalf("pid = %d, want %d", conn.Process.PID, os.Getpid())
			}
			if conn.Direction != "outbound" {
				t.Fatalf("direction = %q, want outbound", conn.Direction)
			}
			return
		}
	}
	t.Fatalf("loopback connection %s -> %s not found in Windows TCP table", local, remote)
}

func TestSampleReportsWindowsCPUAndMemory(t *testing.T) {
	c := New()
	sample, err := c.Sample(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if sample.Platform != "windows" {
		t.Fatalf("platform = %q, want windows", sample.Platform)
	}
	if sample.CPUCores < 1 {
		t.Fatalf("cpu_cores = %d", sample.CPUCores)
	}
	if sample.MemTotalKB == 0 || sample.MemAvailableKB == 0 {
		t.Fatalf("memory sample missing: total=%d available=%d", sample.MemTotalKB, sample.MemAvailableKB)
	}
	if !sample.MetricUnavailable("load_average") {
		t.Fatalf("load_average must be explicitly unavailable on Windows")
	}
	if !sample.MetricUnavailable("io_wait_percent") {
		t.Fatalf("io_wait_percent must be explicitly unavailable on Windows")
	}
}

func TestConnectionsClassifiesListenerSideInbound(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
	}()

	client, err := net.Dial("tcp4", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := <-accepted
	defer server.Close()

	got, err := New().Connections()
	if err != nil {
		t.Fatal(err)
	}
	local := server.LocalAddr().(*net.TCPAddr).AddrPort()
	remote := server.RemoteAddr().(*net.TCPAddr).AddrPort()
	for _, conn := range got {
		if conn.Local == local && conn.Remote == remote {
			if conn.Direction != "inbound" {
				t.Fatalf("direction = %q, want inbound", conn.Direction)
			}
			return
		}
	}
	t.Fatalf("server-side loopback connection %s <- %s not found in Windows TCP table", local, remote)
}
