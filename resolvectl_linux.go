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
	"strings"
	"syscall"
	"time"

	"vea/backend/service/shared"
)

var rootHelperArtifactsRoot string

func deriveArtifactsRootFromSocketPath(socketPath string) (string, error) {
	p := strings.TrimSpace(socketPath)
	if p == "" {
		return "", errors.New("socketPath 为空")
	}
	p = filepath.Clean(p)
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("socketPath 必须是绝对路径: %q", p)
	}

	if filepath.Base(p) != "resolvectl-helper.sock" {
		return "", fmt.Errorf("socketPath 不符合预期: %q", p)
	}
	runtimeDir := filepath.Dir(p)
	if filepath.Base(runtimeDir) != "runtime" {
		return "", fmt.Errorf("socketPath 不符合预期: %q", p)
	}

	root := filepath.Clean(filepath.Dir(runtimeDir))
	if root == "." || root == string(os.PathSeparator) {
		return "", fmt.Errorf("artifactsRoot 不安全: %q", root)
	}

	expected := filepath.Join(root, "runtime", "resolvectl-helper.sock")
	if filepath.Clean(expected) != p {
		return "", fmt.Errorf("socketPath 不符合预期: %q", p)
	}

	return root, nil
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

	// 从 socketPath 推导 ArtifactsRoot：
	// <ArtifactsRoot>/runtime/resolvectl-helper.sock -> <ArtifactsRoot>
	// 用于限制 root helper 的可操作范围（避免对任意路径执行特权动作）。
	artifactsRoot, err := deriveArtifactsRootFromSocketPath(*socketPath)
	if err != nil {
		log.Fatalf("invalid socketPath: %v", err)
	}
	rootHelperArtifactsRoot = artifactsRoot

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
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			if errors.Is(err, syscall.EINTR) {
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
	var req shared.RootHelperRequest
	if err := dec.Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(shared.RootHelperResponse{ExitCode: 1, Error: fmt.Sprintf("解析请求失败: %v", err)})
		return
	}

	resp := dispatchRootHelperRequest(req)
	_ = json.NewEncoder(conn).Encode(resp)
}

func dispatchRootHelperRequest(req shared.RootHelperRequest) shared.RootHelperResponse {
	op := strings.TrimSpace(req.Op)
	if op == "" || op == "resolvectl" {
		return runResolvectlWhitelisted(req.Args)
	}
	switch op {
	case "tun-setup":
		return runTUNSetup(req.BinaryPath)
	case "tun-cleanup":
		return runTUNCleanup()
	default:
		return shared.RootHelperResponse{ExitCode: 1, Error: "不支持的 op"}
	}
}

func runResolvectlWhitelisted(args []string) shared.RootHelperResponse {
	if len(args) == 0 {
		return shared.RootHelperResponse{ExitCode: 1, Error: "参数为空"}
	}
	if args[0] == "__ping" {
		return shared.RootHelperResponse{ExitCode: 0}
	}
	if strings.HasPrefix(args[0], "-") {
		return shared.RootHelperResponse{ExitCode: 1, Error: "不允许传入选项"}
	}

	switch args[0] {
	case "dns", "domain", "default-route", "revert":
	default:
		return shared.RootHelperResponse{ExitCode: 1, Error: "不允许的命令"}
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

	resp := shared.RootHelperResponse{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
	if err != nil && exitCode == 1 {
		resp.Error = err.Error()
	}
	return resp
}

func runTUNCleanup() shared.RootHelperResponse {
	shared.CleanConflictingIPTablesRules()
	return shared.RootHelperResponse{ExitCode: 0}
}

func runTUNSetup(binaryPath string) shared.RootHelperResponse {
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		return shared.RootHelperResponse{ExitCode: 1, Error: "binaryPath 为空"}
	}
	if _, err := os.Stat(binaryPath); err != nil {
		return shared.RootHelperResponse{ExitCode: 1, Error: fmt.Sprintf("binaryPath 无效: %v", err)}
	}

	// 安全：仅允许对本应用 artifacts/core 下的已知内核二进制配置 capabilities，
	// 避免“授权一次后可对任意路径 setcap”的权限扩大。
	root := strings.TrimSpace(rootHelperArtifactsRoot)
	if root == "" {
		return shared.RootHelperResponse{ExitCode: 1, Error: "无法确定 artifactsRoot（socketPath 不符合预期）"}
	}

	realRoot := root
	if p, err := filepath.EvalSymlinks(realRoot); err == nil && p != "" {
		realRoot = p
	}
	if p, err := filepath.Abs(realRoot); err == nil && p != "" {
		realRoot = p
	}
	if realRoot == string(os.PathSeparator) {
		return shared.RootHelperResponse{ExitCode: 1, Error: "artifactsRoot 不安全（解析为 /，拒绝执行）"}
	}

	realBin := binaryPath
	if p, err := filepath.EvalSymlinks(realBin); err == nil && p != "" {
		realBin = p
	}
	if p, err := filepath.Abs(realBin); err == nil && p != "" {
		realBin = p
	}

	rel, err := filepath.Rel(realRoot, realBin)
	if err != nil {
		return shared.RootHelperResponse{ExitCode: 1, Error: fmt.Sprintf("binaryPath 校验失败: %v", err)}
	}
	sep := string(os.PathSeparator)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+sep) {
		return shared.RootHelperResponse{ExitCode: 1, Error: "binaryPath 不在 artifactsRoot 下（拒绝执行）"}
	}
	if !(strings.HasPrefix(rel, "core"+sep+"sing-box"+sep) || strings.HasPrefix(rel, "core"+sep+"clash"+sep)) {
		return shared.RootHelperResponse{ExitCode: 1, Error: "binaryPath 不在允许的 core 目录下（拒绝执行）"}
	}
	switch filepath.Base(realBin) {
	case "sing-box", "mihomo", "clash":
	default:
		return shared.RootHelperResponse{ExitCode: 1, Error: "binaryPath 不是允许的内核二进制名（拒绝执行）"}
	}

	if err := shared.SetupTUNForBinary(realBin); err != nil {
		return shared.RootHelperResponse{ExitCode: 1, Error: err.Error()}
	}
	return shared.RootHelperResponse{ExitCode: 0}
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

	socketPath := strings.TrimSpace(os.Getenv(shared.EnvResolvectlSocket))
	if socketPath == "" {
		execRealResolvectl(args)
		return
	}

	resp, err := shared.CallRootHelper(socketPath, shared.RootHelperRequest{Args: args})
	if err != nil {
		if err2 := shared.EnsureRootHelper(socketPath, shared.RootHelperEnsureOptions{}); err2 == nil {
			resp, err = shared.CallRootHelper(socketPath, shared.RootHelperRequest{Args: args})
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
