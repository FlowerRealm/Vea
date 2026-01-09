package shared

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var DefaultSingBoxRuleSetTags = []string{
	"geosite-category-ads-all",
	"geosite-cn",
	"geoip-cn",
}

// EnsureSingBoxRuleSets 确保 sing-box 所需的 rule-set 文件存在。
//
// 说明：
// - sing-box 的 route.ruleset 本质上依赖本地 .srs 文件；缺失时会在运行期 FATAL。
// - 这里采用“缺什么补什么”的策略，避免用户首次启动就失败。
func EnsureSingBoxRuleSets(tags []string) error {
	if len(tags) == 0 {
		tags = DefaultSingBoxRuleSetTags
	}
	return ensureSingBoxRuleSets(ArtifactsRoot, tags, func(url string) ([]byte, error) {
		data, _, err := DownloadWithProgress(url, nil)
		return data, err
	})
}

// ExtractSingBoxRuleSetTagsFromConfig 从 sing-box 配置 JSON 中提取 route.rule_set 的 tag。
// 仅返回 geosite-/geoip- 前缀的条目，并保持稳定顺序（去重）。
func ExtractSingBoxRuleSetTagsFromConfig(configBytes []byte) ([]string, error) {
	if len(configBytes) == 0 {
		return nil, nil
	}

	var root map[string]interface{}
	if err := json.Unmarshal(configBytes, &root); err != nil {
		return nil, err
	}

	route, _ := root["route"].(map[string]interface{})
	ruleSets, _ := route["rule_set"].([]interface{})
	if len(ruleSets) == 0 {
		return nil, nil
	}

	var tags []string
	seen := make(map[string]struct{}, len(ruleSets))
	for _, rs := range ruleSets {
		m, ok := rs.(map[string]interface{})
		if !ok {
			continue
		}
		tag, _ := m["tag"].(string)
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if !strings.HasPrefix(tag, "geosite-") && !strings.HasPrefix(tag, "geoip-") {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return tags, nil
}

func ensureSingBoxRuleSets(artifactsRoot string, tags []string, download func(url string) ([]byte, error)) error {
	if artifactsRoot == "" {
		return fmt.Errorf("artifacts root is empty")
	}

	ruleSetDir := filepath.Join(artifactsRoot, "core", "sing-box", "rule-set")
	if err := os.MkdirAll(ruleSetDir, 0o755); err != nil {
		return fmt.Errorf("create rule-set dir: %w", err)
	}

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		targetPath := filepath.Join(ruleSetDir, tag+".srs")
		if info, err := os.Stat(targetPath); err == nil && info.Size() > 0 {
			continue
		}

		url, err := singBoxRuleSetURL(tag)
		if err != nil {
			return err
		}

		data, err := download(url)
		if err != nil {
			return fmt.Errorf("download rule-set %s: %w", tag, err)
		}
		if len(data) == 0 {
			return fmt.Errorf("download rule-set %s: empty payload", tag)
		}

		if err := WriteAtomic(targetPath, data, 0o644); err != nil {
			return fmt.Errorf("write rule-set %s: %w", tag, err)
		}
	}

	return nil
}

func singBoxRuleSetURL(tag string) (string, error) {
	tag = strings.ToLower(strings.TrimSpace(tag))
	switch {
	case strings.HasPrefix(tag, "geosite-"):
		return "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/" + tag + ".srs", nil
	case strings.HasPrefix(tag, "geoip-"):
		return "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/" + tag + ".srs", nil
	default:
		return "", fmt.Errorf("unknown sing-box rule-set tag: %s", tag)
	}
}
