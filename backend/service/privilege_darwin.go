// +build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
)

// SetupTUNPrivileges macOS TUN 设置说明
func (s *Service) SetupTUNPrivileges() error {
	return fmt.Errorf("on macOS, TUN mode requires running Vea with sudo")
}

// CheckTUNCapabilities 检查是否有 root 权限
func (s *Service) CheckTUNCapabilities() (bool, error) {
	return os.Geteuid() == 0, nil
}

// StartTUNProcess macOS 下启动 TUN 进程
func (s *Service) StartTUNProcess(binaryPath, configPath string) (*exec.Cmd, error) {
	if os.Geteuid() != 0 {
		// 尝试通过 sudo 启动
		return nil, fmt.Errorf("TUN mode requires root privileges. Please run: sudo vea")
	}

	cmd := exec.Command(binaryPath, "run", "-c", configPath)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start TUN process: %w", err)
	}

	return cmd, nil
}

// CleanupTUNUser macOS 不需要清理
func (s *Service) CleanupTUNUser() error {
	return nil
}
