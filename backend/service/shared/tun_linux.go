//go:build linux
// +build linux

package shared

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
)

const (
	tunUserName  = "vea-tun"
	tunGroupName = "vea-tun"
	// TUN 模式所需的所有 capabilities
	// cap_net_admin: 创建 TUN 设备、设置路由、nftables 规则
	// cap_net_bind_service: 绑定低端口（<1024）
	// cap_net_raw: 使用 SO_BINDTODEVICE 绑定物理接口（bind_interface 需要）
	requiredCapabilities = "cap_net_admin,cap_net_bind_service,cap_net_raw"
)

var ensureTUNOnceMu sync.Mutex
var ensureTUNOnceDone chan struct{}
var ensureTUNOnceNeedRestart bool
var ensureTUNOnceErr error

// CheckTUNCapabilities 检查 Linux TUN 权限是否已配置
func CheckTUNCapabilities() (bool, error) {
	// root 本身就具备创建 TUN/设置路由等权限；不强制要求 capabilities 已配置。
	if os.Geteuid() == 0 {
		return true, nil
	}

	// 检查 vea-tun 用户是否存在
	if _, err := user.Lookup(tunUserName); err != nil {
		log.Printf("[TUN-Check] vea-tun 用户不存在: %v", err)
		return false, nil
	}
	log.Printf("[TUN-Check] vea-tun 用户存在")

	// 检查 sing-box 二进制
	binaryPath, err := FindSingBoxBinary()
	if err != nil {
		log.Printf("[TUN-Check] 获取 sing-box 路径失败: %v", err)
		return false, nil
	}
	log.Printf("[TUN-Check] sing-box 路径: %s", binaryPath)

	// 使用 getcap 检查当前 capabilities
	cmd := exec.Command("getcap", binaryPath)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[TUN-Check] getcap 执行失败: %v", err)
		return false, nil
	}
	log.Printf("[TUN-Check] getcap 输出: %s", string(output))

	// 检查必要的 capabilities
	requiredCaps := []string{"cap_net_admin", "cap_net_bind_service", "cap_net_raw"}
	for _, cap := range requiredCaps {
		if !bytes.Contains(output, []byte(cap)) {
			log.Printf("[TUN-Check] 缺少 capability: %s", cap)
			return false, nil
		}
	}

	log.Printf("[TUN-Check] TUN 权限已完全配置")
	return true, nil
}

// SetupTUN 配置 Linux TUN 权限
func SetupTUN() error {
	// 检查是否以 root 运行
	if os.Geteuid() != 0 {
		return fmt.Errorf("TUN setup requires root privileges")
	}

	// 1. 创建专用用户和组
	if err := ensureTUNUser(); err != nil {
		return fmt.Errorf("failed to create TUN user: %w", err)
	}

	// 2. 设置 sing-box 二进制的 capabilities
	if err := setTUNCapabilities(); err != nil {
		return fmt.Errorf("failed to set capabilities: %w", err)
	}

	return nil
}

// SetupTUNForSingBoxBinary 配置 Linux TUN 权限（指定 sing-box 二进制路径）
func SetupTUNForSingBoxBinary(binaryPath string) error {
	// 检查是否以 root 运行
	if os.Geteuid() != 0 {
		return fmt.Errorf("TUN setup requires root privileges")
	}
	if binaryPath == "" {
		return fmt.Errorf("sing-box binary path is empty")
	}

	// 1. 创建专用用户和组
	if err := ensureTUNUser(); err != nil {
		return fmt.Errorf("failed to create TUN user: %w", err)
	}

	// 2. 设置指定 sing-box 二进制的 capabilities
	if err := setTUNCapabilitiesForBinary(binaryPath); err != nil {
		return fmt.Errorf("failed to set capabilities: %w", err)
	}

	return nil
}

// ensureTUNUser 创建 TUN 专用用户和组
func ensureTUNUser() error {
	// 检查用户是否已存在
	if _, err := user.Lookup(tunUserName); err == nil {
		return nil
	}

	// 创建组
	cmd := exec.Command("groupadd", "-r", tunGroupName)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 9 {
			// 组已存在（错误码 9），忽略
		} else {
			return fmt.Errorf("groupadd failed: %w", err)
		}
	}

	// 创建系统用户（无登录权限）
	cmd = exec.Command("useradd",
		"-r",                      // 系统用户
		"-s", "/usr/sbin/nologin", // 禁止登录
		"-g", tunGroupName, // 主组
		"-M",                      // 不创建家目录
		"-c", "Vea TUN Mode User", // 注释
		tunUserName,
	)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 9 {
			// 用户已存在，忽略
			return nil
		}
		return fmt.Errorf("useradd failed: %w", err)
	}

	return nil
}

// setTUNCapabilities 设置 sing-box 二进制的 capabilities
func setTUNCapabilities() error {
	// 获取 sing-box 二进制路径
	binaryPath, err := FindSingBoxBinary()
	if err != nil {
		return fmt.Errorf("sing-box binary not found: %w", err)
	}

	return setTUNCapabilitiesForBinary(binaryPath)
}

func setTUNCapabilitiesForBinary(binaryPath string) error {
	// 设置文件所有者为 vea-tun 用户
	u, err := user.Lookup(tunUserName)
	if err != nil {
		return fmt.Errorf("user lookup failed: %w", err)
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	if err := os.Chown(binaryPath, uid, gid); err != nil {
		return fmt.Errorf("chown failed: %w", err)
	}

	// chown 会清除文件 capabilities，所以必须在 chown 之后再 setcap。
	cmd := exec.Command("setcap", requiredCapabilities+"+ep", binaryPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("setcap failed: %w", err)
	}

	return nil
}

// TUNCapabilityStatus 描述 TUN 权限状态
type TUNCapabilityStatus struct {
	UserExists      bool     // vea-tun 用户是否存在
	BinaryFound     bool     // sing-box 二进制是否找到
	BinaryPath      string   // sing-box 二进制路径
	CurrentCaps     []string // 当前已有的 capabilities
	MissingCaps     []string // 缺少的 capabilities
	FullyConfigured bool     // 是否完全配置
}

// GetTUNCapabilityStatus 获取详细的 TUN 权限状态（仅 Linux）
func GetTUNCapabilityStatus() TUNCapabilityStatus {
	status := TUNCapabilityStatus{}

	// 1. 检查 vea-tun 用户是否存在
	if _, err := user.Lookup(tunUserName); err == nil {
		status.UserExists = true
	}

	// 2. 检查 sing-box 二进制
	binaryPath, err := FindSingBoxBinary()
	if err != nil {
		return status
	}
	status.BinaryFound = true
	status.BinaryPath = binaryPath

	// 3. 使用 getcap 检查当前 capabilities
	cmd := exec.Command("getcap", binaryPath)
	output, _ := cmd.Output()

	// 解析当前 capabilities
	requiredCaps := []string{"cap_net_admin", "cap_net_bind_service", "cap_net_raw"}
	for _, cap := range requiredCaps {
		if bytes.Contains(output, []byte(cap)) {
			status.CurrentCaps = append(status.CurrentCaps, cap)
		} else {
			status.MissingCaps = append(status.MissingCaps, cap)
		}
	}

	// 4. 判断是否完全配置
	status.FullyConfigured = status.UserExists && status.BinaryFound && len(status.MissingCaps) == 0

	return status
}

// EnsureTUNCapabilities 确保 TUN 权限已配置，缺少则自动修复（仅 Linux）
// 返回 (是否需要重启应用, 错误)
func EnsureTUNCapabilities() (bool, error) {
	// 幂等：避免前端/后台并发触发多次 pkexec，导致多次弹密码框。
	ensureTUNOnceMu.Lock()
	if ensureTUNOnceDone != nil {
		ch := ensureTUNOnceDone
		ensureTUNOnceMu.Unlock()
		<-ch
		ensureTUNOnceMu.Lock()
		needRestart := ensureTUNOnceNeedRestart
		err := ensureTUNOnceErr
		ensureTUNOnceMu.Unlock()
		return needRestart, err
	}
	ensureTUNOnceDone = make(chan struct{})
	ensureTUNOnceMu.Unlock()

	needRestart, err := ensureTUNCapabilitiesOnce()

	ensureTUNOnceMu.Lock()
	ensureTUNOnceNeedRestart = needRestart
	ensureTUNOnceErr = err
	close(ensureTUNOnceDone)
	ensureTUNOnceDone = nil
	ensureTUNOnceMu.Unlock()
	return needRestart, err
}

func ensureTUNCapabilitiesOnce() (bool, error) {
	status := GetTUNCapabilityStatus()

	if status.FullyConfigured {
		return false, nil
	}

	if !status.BinaryFound {
		return false, fmt.Errorf("sing-box 未安装，请先安装 sing-box 组件")
	}

	log.Printf("[TUN-Setup] 检测到 TUN 权限不完整，尝试自动配置...")
	log.Printf("[TUN-Setup] 缺少: user=%v, caps=%v", !status.UserExists, status.MissingCaps)

	binaryPath := status.BinaryPath

	// 重要：一次配置动作里不要分多次 pkexec，否则会多次弹授权。
	// 做法：用 pkexec 运行一次 vea setup-tun（传入 sing-box 路径），在 root 进程内完成 user/group + chown + setcap。
	exePath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	if realPath, err := filepath.EvalSymlinks(exePath); err == nil && realPath != "" {
		exePath = realPath
	}
	if abs, err := filepath.Abs(exePath); err == nil && abs != "" {
		exePath = abs
	}

	log.Printf("[TUN-Setup] 使用 pkexec 调用: %s setup-tun --singbox-binary %s", exePath, binaryPath)
	cmd := exec.Command("pkexec", exePath, "setup-tun", "--singbox-binary", binaryPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("自动配置 TUN 权限失败: %v, stderr: %s", err, stderr.String())
	}

	// 验证配置
	newStatus := GetTUNCapabilityStatus()
	if !newStatus.FullyConfigured {
		return false, fmt.Errorf("自动配置后权限仍不完整: missingCaps=%v\n请手动运行: sudo setcap '%s+ep' %s",
			newStatus.MissingCaps, requiredCapabilities, binaryPath)
	}

	log.Printf("[TUN-Setup] TUN 权限配置完成")
	return false, nil
}
