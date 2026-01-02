package adapters

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestSingBoxAdapter_WaitForReady_PortInUseReturnsNil(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		t.Fatalf("unexpected addr: %T %v", ln.Addr(), ln.Addr())
	}

	a := &SingBoxAdapter{}
	handle := &ProcessHandle{Port: addr.Port}
	if err := a.WaitForReady(handle, 300*time.Millisecond); err != nil {
		t.Fatalf("WaitForReady() expected nil, got %v", err)
	}
}

func TestSingBoxAdapter_WaitForReady_PortFreeTimesOut(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		_ = ln.Close()
		t.Fatalf("unexpected addr: %T %v", ln.Addr(), ln.Addr())
	}
	port := addr.Port
	_ = ln.Close()

	a := &SingBoxAdapter{}
	handle := &ProcessHandle{Port: port}
	err = a.WaitForReady(handle, 200*time.Millisecond)
	if err == nil {
		t.Fatalf("WaitForReady() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestFormatSingBoxDurationSeconds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		seconds int
		want    string
	}{
		{seconds: 0, want: ""},
		{seconds: -1, want: ""},
		{seconds: 1, want: "1s"},
		{seconds: 60, want: "1m"},
		{seconds: 120, want: "2m"},
		{seconds: 90, want: "90s"},
		{seconds: 3600, want: "1h"},
		{seconds: 7200, want: "2h"},
	}

	for _, tt := range cases {
		if got := formatSingBoxDurationSeconds(tt.seconds); got != tt.want {
			t.Fatalf("formatSingBoxDurationSeconds(%d)=%q want %q", tt.seconds, got, tt.want)
		}
	}
}
