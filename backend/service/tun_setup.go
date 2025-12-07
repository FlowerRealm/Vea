package service

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"

	"vea/backend/domain"
)

// getCurrentWorkingDir 获取当前工作目录
func getCurrentWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "/path/to/vea"
	}
	return dir
}

// SetupTUN 配置 TUN 模式（需要以管理员/root 身份运行）
func (s *Service) SetupTUN() error {
	switch runtime.GOOS {
	case "linux":
		return s.setupTUNLinux()
	case "windows":
		return s.setupTUNWindows()
	case "darwin":
		return s.setupTUNDarwin()
	default:
		return fmt.Errorf("TUN mode is not supported on %s", runtime.GOOS)
	}
}

func (s *Service) setupTUNLinux() error {
	// 检查是否以 root 身份运行
	if os.Getuid() != 0 {
		return fmt.Errorf("TUN 模式需要管理员权限。请使用 root 权限启动应用。")
	}

	log.Printf("[TUN-Setup] 检测到 root 权限，开始配置...")

	// 1. 创建 vea-tun 用户
	cmd := exec.Command("useradd", "-r", "-s", "/usr/sbin/nologin", "-M", "vea-tun")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	stderrStr := stderr.String()

	// 检查是否因为用户已存在而失败
	userExists := bytes.Contains(stderr.Bytes(), []byte("already exists")) ||
		bytes.Contains(stderr.Bytes(), []byte("已存在"))

	if err != nil && !userExists {
		// 真正的错误，不是"用户已存在"
		return fmt.Errorf("failed to create vea-tun user: %w, stderr: %s", err, stderrStr)
	}

	if userExists {
		log.Printf("[TUN-Setup] vea-tun 用户已存在，继续配置权限")
	} else {
		log.Printf("[TUN-Setup] vea-tun 用户已创建")
	}

	// 2. 设置 sing-box 二进制的 capabilities
	binaryPath, err := s.getEngineBinaryPath(domain.EngineSingBox)
	if err != nil {
		// sing-box 未安装，只创建用户即可
		log.Printf("[TUN-Setup] sing-box 未安装，稍后安装时需要重新配置")
		return nil
	}

	log.Printf("[TUN-Setup] 设置 sing-box 权限: %s", binaryPath)

	// 先设置二进制文件所有者为 vea-tun（必须在 setcap 之前，因为 chown 会清除 capabilities）
	cmd = exec.Command("chown", "vea-tun:vea-tun", binaryPath)
	stderr.Reset()
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to chown binary: %w, stderr: %s", err, stderr.String())
	}
	log.Printf("[TUN-Setup] chown 完成")

	// 3. 设置 capabilities（在 chown 之后）
	// TUN 模式所需的 capabilities：
	// cap_net_admin: 创建 TUN 设备、设置路由、nftables 规则
	// cap_net_bind_service: 绑定低端口（<1024）
	// cap_net_raw: 使用 SO_BINDTODEVICE 绑定物理接口（bind_interface 需要）
	caps := "cap_net_admin,cap_net_bind_service,cap_net_raw"
	cmd = exec.Command("setcap", caps+"+ep", binaryPath)
	stderr.Reset()
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set capabilities: %w, stderr: %s", err, stderr.String())
	}
	log.Printf("[TUN-Setup] setcap 完成")

	log.Printf("[TUN-Setup] TUN 配置完成")
	return nil
}

func (s *Service) setupTUNWindows() error {
	// Windows 需要以管理员身份运行应用
	configured, err := s.CheckTUNCapabilities()
	if err != nil {
		return fmt.Errorf("failed to check administrator privileges: %w", err)
	}

	if !configured {
		return fmt.Errorf("TUN mode requires administrator privileges.\n" +
			"Please right-click Vea and select 'Run as administrator'")
	}

	log.Printf("[TUN-Setup] Windows 管理员权限已确认")
	return nil
}

func (s *Service) setupTUNDarwin() error {
	// macOS 需要以 root 身份运行
	if os.Getuid() != 0 {
		return fmt.Errorf("TUN setup requires root privileges.\n" +
			"Please run Vea with: sudo /Applications/Vea.app/Contents/MacOS/Vea")
	}

	log.Printf("[TUN-Setup] macOS root 权限已确认")
	return nil
}

// CleanupTUN 清理 TUN 配置（需要以管理员/root 身份运行）
func (s *Service) CleanupTUN() error {
	switch runtime.GOOS {
	case "linux":
		return s.cleanupTUNLinux()
	case "windows", "darwin":
		return nil // 不需要清理
	default:
		return nil
	}
}

func (s *Service) cleanupTUNLinux() error {
	// 检查是否以 root 身份运行
	if os.Getuid() != 0 {
		return fmt.Errorf("TUN cleanup requires root privileges. Please run Vea as administrator:\n" +
			"  - Use 'Vea (管理员模式)' desktop shortcut, or\n" +
			"  - Run: pkexec /usr/bin/vea-admin")
	}

	log.Printf("[TUN-Cleanup] 检测到 root 权限，开始清理...")

	var stderr bytes.Buffer

	// 1. 恢复 sing-box 二进制的权限（如果存在）
	binaryPath, err := s.getEngineBinaryPath(domain.EngineSingBox)
	if err == nil {
		log.Printf("[TUN-Cleanup] 清理 sing-box 权限: %s", binaryPath)

		// 移除 capabilities
		cmd := exec.Command("setcap", "-r", binaryPath)
		stderr.Reset()
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			log.Printf("[TUN-Cleanup] 警告：移除 capabilities 失败: %v, stderr: %s", err, stderr.String())
		} else {
			log.Printf("[TUN-Cleanup] capabilities 已移除")
		}

		// 恢复文件所有者（获取原始用户 ID）
		// 注意：当以 root 运行时，需要从环境变量获取原始用户
		originalUID := os.Getenv("SUDO_UID")
		originalGID := os.Getenv("SUDO_GID")
		if originalUID == "" || originalGID == "" {
			// 如果不是通过 sudo 运行，则恢复为 root
			originalUID = "0"
			originalGID = "0"
		}

		cmd = exec.Command("chown", fmt.Sprintf("%s:%s", originalUID, originalGID), binaryPath)
		stderr.Reset()
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			log.Printf("[TUN-Cleanup] 警告：恢复文件所有者失败: %v, stderr: %s", err, stderr.String())
		} else {
			log.Printf("[TUN-Cleanup] 文件所有者已恢复")
		}
	} else {
		log.Printf("[TUN-Cleanup] sing-box 未安装，跳过权限清理")
	}

	// 2. 删除用户
	cmd := exec.Command("userdel", "vea-tun")
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// 检查是否因为用户不存在而失败
		if !bytes.Contains(stderr.Bytes(), []byte("does not exist")) &&
			!bytes.Contains(stderr.Bytes(), []byte("不存在")) {
			log.Printf("[TUN-Cleanup] 警告：删除用户失败: %v, stderr: %s", err, stderr.String())
		} else {
			log.Printf("[TUN-Cleanup] vea-tun 用户不存在")
		}
	} else {
		log.Printf("[TUN-Cleanup] vea-tun 用户已删除")
	}

	// 3. 删除组
	cmd = exec.Command("groupdel", "vea-tun")
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// 检查是否因为组不存在而失败
		if !bytes.Contains(stderr.Bytes(), []byte("does not exist")) &&
			!bytes.Contains(stderr.Bytes(), []byte("不存在")) {
			log.Printf("[TUN-Cleanup] 警告：删除组失败: %v, stderr: %s", err, stderr.String())
		} else {
			log.Printf("[TUN-Cleanup] vea-tun 组不存在")
		}
	} else {
		log.Printf("[TUN-Cleanup] vea-tun 组已删除")
	}

	log.Printf("[TUN-Cleanup] TUN 配置已清理")
	return nil
}
