// +build windows

package service

import (
	"fmt"
	"os/exec"
	"syscall"
)

var (
	shell32               = syscall.NewLazyDLL("shell32.dll")
	procIsUserAnAdmin     = shell32.NewProc("IsUserAnAdmin")
)

// SetupTUNPrivileges Windows 不需要特殊设置
func (s *Service) SetupTUNPrivileges() error {
	return fmt.Errorf("TUN setup is automatic on Windows (requires running as Administrator)")
}

// CheckTUNCapabilities 检查是否以管理员身份运行
func (s *Service) CheckTUNCapabilities() (bool, error) {
	return isAdmin()
}

// isAdmin 检查当前进程是否以管理员身份运行
func isAdmin() (bool, error) {
	ret, _, _ := procIsUserAnAdmin.Call()
	return ret != 0, nil
}

// StartTUNProcess Windows 下直接启动进程（需要以管理员运行主程序）
func (s *Service) StartTUNProcess(binaryPath, configPath string) (*exec.Cmd, error) {
	if admin, _ := isAdmin(); !admin {
		return nil, fmt.Errorf("TUN mode requires administrator privileges. Please run Vea as Administrator")
	}

	cmd := exec.Command(binaryPath, "run", "-c", configPath)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start TUN process: %w", err)
	}

	return cmd, nil
}

// CleanupTUNUser Windows 不需要清理
func (s *Service) CleanupTUNUser() error {
	return nil
}

// EnsureTUNCapabilities Windows 下检查是否有管理员权限
// 返回 (是否需要重启应用, 错误)
func (s *Service) EnsureTUNCapabilities() (bool, error) {
	if admin, _ := isAdmin(); !admin {
		return false, fmt.Errorf("TUN mode requires administrator privileges. Please run Vea as Administrator")
	}
	return false, nil
}
