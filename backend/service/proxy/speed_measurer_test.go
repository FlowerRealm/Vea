package proxy

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"vea/backend/domain"
	"vea/backend/service/shared"
)

func TestValidateNodeForProbe_InvalidPort(t *testing.T) {
	t.Parallel()

	t.Skip("node-level probe has been removed; FRouter measurement is end-to-end now")
}

func TestInstalledEnginesFromComponents_DetectsByKind(t *testing.T) {
	t.Parallel()

	engines := installedEnginesFromComponents([]domain.CoreComponent{
		{
			Name:            "sing-box",
			Kind:            domain.ComponentSingBox,
			InstallDir:      "/tmp/sing-box",
			LastInstalledAt: time.Now(),
		},
	})
	if _, ok := engines[domain.EngineSingBox]; !ok {
		t.Fatalf("expected sing-box to be detected as installed")
	}
}

func TestEngineFromComponent_UnknownKind(t *testing.T) {
	t.Parallel()

	engine, ok := engineFromComponent(domain.CoreComponent{
		Name:       "unknown",
		Kind:       "",
		InstallDir: "/tmp/unknown",
	})
	if ok || engine != "" {
		t.Fatalf("expected no engine for unknown kind, got engine=%q ok=%v", engine, ok)
	}
}

func TestFindBinaryInDir_FindsInSubdir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sub := filepath.Join(dir, "sing-box-1.0.0-linux-amd64")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	bin := filepath.Join(sub, "sing-box")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	got, err := shared.FindBinaryInDir(dir, []string{"sing-box", "sing-box.exe"})
	if err != nil {
		t.Fatalf("expected to find binary, got err: %v", err)
	}
	if got != bin {
		t.Fatalf("expected %q, got %q", bin, got)
	}
}

func TestDownloadViaSocks5OnceFastResponseClampsElapsed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

		handshake := make([]byte, 3)
		if _, err := io.ReadFull(conn, handshake); err != nil {
			return
		}
		_, _ = conn.Write([]byte{0x05, 0x00})

		hdr := make([]byte, 4)
		if _, err := io.ReadFull(conn, hdr); err != nil {
			return
		}
		switch hdr[3] {
		case 0x01:
			_, _ = io.ReadFull(conn, make([]byte, 4))
		case 0x03:
			lenBuf := make([]byte, 1)
			if _, err := io.ReadFull(conn, lenBuf); err != nil {
				return
			}
			_, _ = io.ReadFull(conn, make([]byte, int(lenBuf[0])))
		case 0x04:
			_, _ = io.ReadFull(conn, make([]byte, 16))
		default:
			return
		}
		_, _ = io.ReadFull(conn, make([]byte, 2))

		_, _ = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		buf := make([]byte, 1024)
		_, _ = conn.Read(buf)
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 4\r\n\r\nTEST"))
	}()

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	minSeconds := 0.5
	bytesRead, seconds, err := downloadViaSocks5Once(ctx, host, port, socksTarget{
		host:  "example.com",
		port:  80,
		path:  "/10MB.zip",
		tls:   false,
		bytes: 0,
	}, minSeconds, nil)
	<-serverDone
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if seconds < minSeconds {
		t.Fatalf("expected elapsed >= %v, got %v", minSeconds, seconds)
	}
	if bytesRead == 0 {
		t.Fatalf("expected to read some bytes")
	}
}

func TestMeasureDownloadFixedDurationWith_UsesFixedDurationAndWorkers(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	download := func(ctx context.Context, _ string, _ int, _ socksTarget, _ float64, _ func(int64, float64)) (int64, float64, error) {
		calls.Add(1)
		<-ctx.Done()
		// 固定时长测速：ctx deadline 是正常停止信号，返回 ctx.Err() 也应被忽略。
		return 1_000_000, 0, ctx.Err()
	}

	duration := 50 * time.Millisecond
	workers := 4
	mbps, err := measureDownloadFixedDurationWith(context.Background(), "", 0, []socksTarget{{host: "example.com", port: 80, path: "/", tls: false}}, duration, workers, download, nil)
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if got := calls.Load(); got != int64(workers) {
		t.Fatalf("expected %d workers, got %d calls", workers, got)
	}

	expected := (float64(workers*1_000_000) / duration.Seconds()) / (1024 * 1024)
	if (mbps-expected) > 0.0001 || (expected-mbps) > 0.0001 {
		t.Fatalf("expected %.6f, got %.6f", expected, mbps)
	}
}

func TestMeasureDownloadFixedDurationWith_NoDataReturnsError(t *testing.T) {
	t.Parallel()

	download := func(ctx context.Context, _ string, _ int, _ socksTarget, _ float64, _ func(int64, float64)) (int64, float64, error) {
		<-ctx.Done()
		return 0, 0, ctx.Err()
	}

	_, err := measureDownloadFixedDurationWith(context.Background(), "", 0, []socksTarget{{host: "example.com", port: 80, path: "/", tls: false}}, 20*time.Millisecond, 2, download, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestMeasureDownloadFixedDurationWith_EOFIsRetryable(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	download := func(ctx context.Context, _ string, _ int, _ socksTarget, _ float64, _ func(int64, float64)) (int64, float64, error) {
		if calls.Add(1) == 1 {
			return 0, 0, io.EOF
		}
		<-ctx.Done()
		return 123, 0, ctx.Err()
	}

	mbps, err := measureDownloadFixedDurationWith(
		context.Background(),
		"",
		0,
		[]socksTarget{{host: "example.com", port: 80, path: "/", tls: false}},
		30*time.Millisecond,
		1,
		download,
		nil,
	)
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if mbps <= 0 {
		t.Fatalf("expected mbps > 0, got %.6f", mbps)
	}
}

func TestBuildSocks5ConnectRequest_IPv4AndIPv6ATYP(t *testing.T) {
	t.Parallel()

	req, err := buildSocks5ConnectRequest("1.2.3.4", 80)
	if err != nil {
		t.Fatalf("buildSocks5ConnectRequest() error: %v", err)
	}
	if len(req) != 10 {
		t.Fatalf("expected ipv4 request length 10, got %d", len(req))
	}
	if req[3] != 0x01 {
		t.Fatalf("expected ipv4 atyp=0x01, got 0x%02x", req[3])
	}

	req6, err := buildSocks5ConnectRequest("2001:db8::1", 443)
	if err != nil {
		t.Fatalf("buildSocks5ConnectRequest() ipv6 error: %v", err)
	}
	if len(req6) != 22 {
		t.Fatalf("expected ipv6 request length 22, got %d", len(req6))
	}
	if req6[3] != 0x04 {
		t.Fatalf("expected ipv6 atyp=0x04, got 0x%02x", req6[3])
	}
	if req6[len(req6)-2] != 0x01 || req6[len(req6)-1] != 0xbb {
		t.Fatalf("expected port 443 in last two bytes, got 0x%02x 0x%02x", req6[len(req6)-2], req6[len(req6)-1])
	}
}
