package service

import (
	"log"
	"os/exec"
	"runtime"
	"strings"
)

func (s *Service) cleanupTUN(interfaceName string) error {
	log.Printf("[TUN] 正在清理接口 %s 和相关路由...", interfaceName)

	// 0. 清理 nftables 规则（auto_redirect 创建的）
	// sing-box 使用 nftables 的 "sing-box" 表来实现 auto_redirect
	if runtime.GOOS == "linux" {
		cleanupNftables()
	}

	// 1. 删除接口 (如果存在)
	// ip link delete tun0
	cmd := exec.Command("ip", "link", "delete", interfaceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		// 如果接口不存在，忽略错误
		if !strings.Contains(string(out), "Cannot find device") {
			log.Printf("[TUN] 删除接口失败: %v, output: %s", err, string(out))
		}
	} else {
		log.Printf("[TUN] 接口 %s 已删除", interfaceName)
	}

	// 2. 清理 Sing-box 可能残留的策略路由
	// ip rule del from all lookup 2022
	// 尝试多次删除，直到报错（说明删干净了）
	for i := 0; i < 10; i++ {
		cmd := exec.Command("ip", "rule", "del", "from", "all", "lookup", "2022")
		if err := cmd.Run(); err != nil {
			break
		}
		log.Printf("[TUN] 已删除一条残留的策略路由 (lookup 2022)")
	}

	// 3. 清理默认路由残留 (add route 0: file exists)
	// 有时候 auto_route 会尝试添加 default 路由到 table 2022，如果残留可能导致冲突
	// ip route flush table 2022
	cmd = exec.Command("ip", "route", "flush", "table", "2022")
	if err := cmd.Run(); err != nil {
		log.Printf("[TUN] 清理路由表 2022 失败: %v", err)
	} else {
		log.Printf("[TUN] 路由表 2022 已清理")
	}

	return nil
}

// cleanupNftables 清理 sing-box auto_redirect 创建的 nftables 规则
func cleanupNftables() {
	// sing-box 创建的 nftables 表名为 "sing-box"
	// 删除 inet 表（同时处理 IPv4 和 IPv6）
	tables := []struct {
		family string
		name   string
	}{
		{"inet", "sing-box"},
		{"ip", "sing-box"},
		{"ip6", "sing-box"},
	}

	for _, t := range tables {
		cmd := exec.Command("nft", "delete", "table", t.family, t.name)
		if out, err := cmd.CombinedOutput(); err != nil {
			// 如果表不存在，忽略错误
			outStr := string(out)
			if !strings.Contains(outStr, "No such file") && !strings.Contains(outStr, "does not exist") {
				log.Printf("[TUN] 清理 nftables 表 %s %s 失败: %v", t.family, t.name, err)
			}
		} else {
			log.Printf("[TUN] 已清理 nftables 表 %s %s", t.family, t.name)
		}
	}
}
