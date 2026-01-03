//go:build windows
// +build windows

package shared

import (
	"fmt"
	"syscall"
)

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procIsUserAnAdmin = shell32.NewProc("IsUserAnAdmin")
)

// CheckTUNCapabilities 检查 Windows TUN 权限（管理员权限）
func CheckTUNCapabilities() (bool, error) {
	return isAdmin()
}

// SetupTUN Windows 不需要特殊设置
func SetupTUN() error {
	return fmt.Errorf("TUN setup is automatic on Windows (requires running as Administrator)")
}

// SetupTUNForSingBoxBinary Windows stub（仅为跨平台编译提供一致接口）
func SetupTUNForSingBoxBinary(binaryPath string) error {
	return SetupTUN()
}

// isAdmin 检查当前进程是否以管理员身份运行
func isAdmin() (bool, error) {
	ret, _, _ := procIsUserAnAdmin.Call()
	return ret != 0, nil
}

// TUNCapabilityStatus Windows stub
type TUNCapabilityStatus struct {
	UserExists      bool
	BinaryFound     bool
	BinaryPath      string
	CurrentCaps     []string
	MissingCaps     []string
	FullyConfigured bool
}

// GetTUNCapabilityStatus Windows 版本
func GetTUNCapabilityStatus() TUNCapabilityStatus {
	admin, _ := isAdmin()
	return TUNCapabilityStatus{
		FullyConfigured: admin,
	}
}

// EnsureTUNCapabilities Windows 下检查是否有管理员权限
func EnsureTUNCapabilities() (bool, error) {
	if admin, _ := isAdmin(); !admin {
		return false, fmt.Errorf("TUN mode requires administrator privileges. Please run Vea as Administrator")
	}
	return false, nil
}
