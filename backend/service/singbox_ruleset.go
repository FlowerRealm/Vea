package service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// downloadSingBoxRuleSets 下载 Sing-box rule-set 文件
func (s *Service) downloadSingBoxRuleSets(installDir string) error {
	// 创建 rule-set 目录
	ruleSetDir := filepath.Join(installDir, "rule-set")
	if err := os.MkdirAll(ruleSetDir, 0755); err != nil {
		return fmt.Errorf("创建 rule-set 目录失败: %w", err)
	}

	// 定义需要下载的 rule-set 列表
	ruleSets := []struct {
		name string
		url  string
	}{
		{
			name: "geosite-category-ads-all.srs",
			url:  "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-category-ads-all.srs",
		},
		{
			name: "geosite-cn.srs",
			url:  "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs",
		},
		{
			name: "geoip-cn.srs",
			url:  "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs",
		},
	}

	// 下载每个 rule-set 文件
	for _, rs := range ruleSets {
		filePath := filepath.Join(ruleSetDir, rs.name)
		log.Printf("[Sing-box] 下载 %s...", rs.name)

		data, _, err := downloadResource(rs.url)
		if err != nil {
			return fmt.Errorf("下载 %s 失败: %w", rs.name, err)
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", rs.name, err)
		}

		log.Printf("[Sing-box] %s 下载完成 (%d bytes)", rs.name, len(data))
	}

	log.Printf("[Sing-box] 所有 rule-set 文件下载完成，保存至: %s", ruleSetDir)
	return nil
}
