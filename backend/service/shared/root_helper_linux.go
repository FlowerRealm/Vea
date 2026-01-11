//go:build linux
// +build linux

package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RootHelperRequest 是 pkexec 启动的 root helper 的 IPC 请求。
//
// 约定:
// - Op 为空时，默认视为 "resolvectl"（兼容旧协议）。
// - Op == "resolvectl" 时，Args 是 resolvectl 的 argv（不含二进制名）。
type RootHelperRequest struct {
	Op         string   `json:"op,omitempty"`
	Args       []string `json:"args,omitempty"`
	BinaryPath string   `json:"binaryPath,omitempty"`
}

type RootHelperResponse struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Error    string `json:"error,omitempty"`
}

type RootHelperEnsureOptions struct {
	// VeaPath 是用于 pkexec 启动 helper 的 vea 可执行文件路径。
	// 为空时依次回退：EnvVeaExecutable → os.Executable()。
	VeaPath string

	// UID 用于 chown socket 文件，使调用者用户可连接。
	// 为 0 时依次回退：EnvResolvectlUID → os.Getuid()。
	UID int

	// ParentPID 会被 helper 监控；当该进程退出时，helper 也会退出。
	// 为 0 时依次回退：EnvResolvectlHelperParent → os.Getppid()。
	ParentPID int
}

func CallRootHelper(socketPath string, req RootHelperRequest) (RootHelperResponse, error) {
	conn, err := net.DialTimeout("unix", socketPath, 200*time.Millisecond)
	if err != nil {
		return RootHelperResponse{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return RootHelperResponse{}, err
	}

	var resp RootHelperResponse
	if err := json.NewDecoder(io.LimitReader(conn, 1024*1024)).Decode(&resp); err != nil {
		return RootHelperResponse{}, err
	}
	return resp, nil
}

func EnsureRootHelper(socketPath string, options RootHelperEnsureOptions) error {
	lockPath := socketPath + ".lock"
	lockFd, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := CallRootHelper(socketPath, RootHelperRequest{Args: []string{"__ping"}}); err == nil {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return fmt.Errorf("等待 helper 超时: %w", err)
	}
	_ = lockFd.Close()
	defer os.Remove(lockPath)

	veaPath := strings.TrimSpace(options.VeaPath)
	if veaPath == "" {
		veaPath = strings.TrimSpace(os.Getenv(EnvVeaExecutable))
	}
	if veaPath == "" {
		if exe, err := os.Executable(); err == nil {
			veaPath = exe
		}
	}
	if veaPath == "" {
		return errors.New("缺少 vea 可执行文件路径：EnvVeaExecutable 与 os.Executable() 均获取失败")
	}
	if realPath, err := filepath.EvalSymlinks(veaPath); err == nil && realPath != "" {
		veaPath = realPath
	}

	uid := options.UID
	if uid == 0 {
		if uidStr := strings.TrimSpace(os.Getenv(EnvResolvectlUID)); uidStr != "" {
			if parsed, err := strconv.Atoi(uidStr); err == nil && parsed > 0 {
				uid = parsed
			}
		}
	}
	if uid == 0 {
		uid = os.Getuid()
	}

	parentPID := options.ParentPID
	if parentPID == 0 {
		if parentStr := strings.TrimSpace(os.Getenv(EnvResolvectlHelperParent)); parentStr != "" {
			if parsed, err := strconv.Atoi(parentStr); err == nil && parsed > 0 {
				parentPID = parsed
			}
		}
	}
	if parentPID == 0 {
		parentPID = os.Getppid()
	}

	pkexecPath, err := exec.LookPath("pkexec")
	if err != nil {
		return fmt.Errorf("未找到 pkexec: %w", err)
	}

	cmd := exec.Command(pkexecPath, veaPath, "resolvectl-helper",
		"--socket", socketPath,
		"--uid", strconv.Itoa(uid),
		"--parent-pid", strconv.Itoa(parentPID),
	)

	// 除非 helper 内部显式输出，否则保持静默。
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("通过 pkexec 启动 helper 失败: %w", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := CallRootHelper(socketPath, RootHelperRequest{Args: []string{"__ping"}}); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("helper 未在预期时间内就绪")
}
