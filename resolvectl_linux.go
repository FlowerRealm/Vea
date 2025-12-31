//go:build linux
// +build linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type resolvectlRequest struct {
	Args []string `json:"args"`
}

type resolvectlResponse struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Error    string `json:"error,omitempty"`
}

func runResolvectlHelper() {
	fs := flag.NewFlagSet("resolvectl-helper", flag.ContinueOnError)
	socketPath := fs.String("socket", "", "unix socket path")
	uid := fs.Int("uid", -1, "uid to chown socket file")
	parentPID := fs.Int("parent-pid", 0, "parent process pid to watch")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}

	if os.Geteuid() != 0 {
		log.Fatal("resolvectl-helper requires root privileges")
	}
	if strings.TrimSpace(*socketPath) == "" {
		log.Fatal("--socket is required")
	}
	if *uid < 0 {
		log.Fatal("--uid is required")
	}

	if err := os.MkdirAll(filepath.Dir(*socketPath), 0o755); err != nil {
		log.Fatalf("mkdir socket dir: %v", err)
	}
	_ = os.Remove(*socketPath)

	ln, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("listen unix socket: %v", err)
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(*socketPath)
	}()

	if err := os.Chown(*socketPath, *uid, -1); err != nil {
		log.Fatalf("chown socket: %v", err)
	}
	if err := os.Chmod(*socketPath, 0o600); err != nil {
		log.Fatalf("chmod socket: %v", err)
	}

	stop := make(chan struct{})
	if *parentPID > 0 {
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				<-ticker.C
				if !processAlive(*parentPID) {
					close(stop)
					return
				}
			}
		}()
		go func() {
			<-stop
			_ = ln.Close()
		}()
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-stop:
				return
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			log.Fatalf("accept: %v", err)
		}
		go handleResolvectlConn(conn)
	}
}

func handleResolvectlConn(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	dec := json.NewDecoder(io.LimitReader(conn, 64*1024))
	var req resolvectlRequest
	if err := dec.Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(resolvectlResponse{ExitCode: 1, Error: fmt.Sprintf("decode request: %v", err)})
		return
	}

	resp := runResolvectlWhitelisted(req.Args)
	_ = json.NewEncoder(conn).Encode(resp)
}

func runResolvectlWhitelisted(args []string) resolvectlResponse {
	if len(args) == 0 {
		return resolvectlResponse{ExitCode: 1, Error: "empty args"}
	}
	if args[0] == "__ping" {
		return resolvectlResponse{ExitCode: 0}
	}
	if strings.HasPrefix(args[0], "-") {
		return resolvectlResponse{ExitCode: 1, Error: "options are not allowed"}
	}

	switch args[0] {
	case "dns", "domain", "default-route", "revert":
	default:
		return resolvectlResponse{ExitCode: 1, Error: "command not allowed"}
	}

	resolvectlPath := "/usr/bin/resolvectl"
	if _, err := os.Stat(resolvectlPath); err != nil {
		if lp, err2 := exec.LookPath("resolvectl"); err2 == nil && lp != "" {
			resolvectlPath = lp
		}
	}

	cmd := exec.Command(resolvectlPath, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	resp := resolvectlResponse{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
	if err != nil && exitCode == 1 {
		resp.Error = err.Error()
	}
	return resp
}

func processAlive(pid int) bool {
	if pid <= 1 {
		return true
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	return true
}

func runResolvectlShim() {
	args := os.Args[2:]

	socketPath := strings.TrimSpace(os.Getenv("VEA_RESOLVECTL_SOCKET"))
	if socketPath == "" {
		execRealResolvectl(args)
		return
	}

	resp, err := callResolvectlHelper(socketPath, args)
	if err != nil {
		if err2 := ensureResolvectlHelper(socketPath); err2 == nil {
			resp, err = callResolvectlHelper(socketPath, args)
		}
	}

	if err != nil {
		execRealResolvectl(args)
		return
	}

	if resp.Stdout != "" {
		_, _ = io.WriteString(os.Stdout, resp.Stdout)
	}
	if resp.Stderr != "" {
		_, _ = io.WriteString(os.Stderr, resp.Stderr)
	}
	if resp.Error != "" && resp.ExitCode == 0 {
		_, _ = fmt.Fprintln(os.Stderr, resp.Error)
		os.Exit(1)
	}
	os.Exit(resp.ExitCode)
}

func execRealResolvectl(args []string) {
	real := "/usr/bin/resolvectl"
	if _, err := os.Stat(real); err != nil {
		if lp, err2 := exec.LookPath("resolvectl"); err2 == nil && lp != "" {
			real = lp
		}
	}
	cmd := exec.Command(real, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func callResolvectlHelper(socketPath string, args []string) (resolvectlResponse, error) {
	conn, err := net.DialTimeout("unix", socketPath, 200*time.Millisecond)
	if err != nil {
		return resolvectlResponse{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if err := json.NewEncoder(conn).Encode(resolvectlRequest{Args: args}); err != nil {
		return resolvectlResponse{}, err
	}

	var resp resolvectlResponse
	if err := json.NewDecoder(io.LimitReader(conn, 1024*1024)).Decode(&resp); err != nil {
		return resolvectlResponse{}, err
	}
	return resp, nil
}

func ensureResolvectlHelper(socketPath string) error {
	lockPath := socketPath + ".lock"
	lockFd, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := callResolvectlHelper(socketPath, []string{"__ping"}); err == nil {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return fmt.Errorf("timeout waiting helper: %w", err)
	}
	_ = lockFd.Close()
	defer os.Remove(lockPath)

	veaPath := strings.TrimSpace(os.Getenv("VEA_EXECUTABLE"))
	if veaPath == "" {
		if exe, err := os.Executable(); err == nil {
			veaPath = exe
		}
	}
	if veaPath == "" {
		return errors.New("missing VEA_EXECUTABLE and os.Executable failed")
	}
	if realPath, err := filepath.EvalSymlinks(veaPath); err == nil && realPath != "" {
		veaPath = realPath
	}

	uidStr := strings.TrimSpace(os.Getenv("VEA_RESOLVECTL_UID"))
	if uidStr == "" {
		uidStr = strconv.Itoa(os.Getuid())
	}

	parentStr := strings.TrimSpace(os.Getenv("VEA_RESOLVECTL_PARENT_PID"))
	if parentStr == "" {
		parentStr = strconv.Itoa(os.Getppid())
	}

	pkexecPath, err := exec.LookPath("pkexec")
	if err != nil {
		return fmt.Errorf("pkexec not found: %w", err)
	}

	cmd := exec.Command(pkexecPath, veaPath, "resolvectl-helper",
		"--socket", socketPath,
		"--uid", uidStr,
		"--parent-pid", parentStr,
	)

	// 不要继承 shim 的 stdout/stderr：sing-box 会把输出当成 resolvectl 的输出解析。
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start resolvectl-helper via pkexec: %w", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := callResolvectlHelper(socketPath, []string{"__ping"}); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("helper not ready in time")
}
