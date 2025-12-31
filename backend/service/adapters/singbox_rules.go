package adapters

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"vea/backend/domain"
)

// RuleSetEntry 表示一个 sing-box rule-set 条目
type RuleSetEntry struct {
	Tag    string // rule-set 标签
	Type   string // local 或 remote
	Format string // binary 或 source
	Path   string // 本地路径（type=local 时）
	URL    string // 远程 URL（type=remote 时）
}

// RoutingRuleEntry 表示一个 sing-box 路由规则条目
type RoutingRuleEntry struct {
	RuleSet       []string // rule_set 匹配
	Domain        []string // domain 精确匹配
	DomainSuffix  []string // domain_suffix 后缀匹配
	DomainKeyword []string // domain_keyword 关键字匹配
	DomainRegex   []string // domain_regex 正则匹配
	IPCidr        []string // ip_cidr 匹配
	Protocol      []string // protocol 匹配
	Port          []int    // port 匹配
	ProcessName   []string // process_name 匹配
	IPIsPrivate   bool     // ip_is_private
	Outbound      string   // 出站标签
}

// RuleSetManager 管理 sing-box rule-set
type RuleSetManager struct {
	artifactsDir string
	ruleSets     map[string]RuleSetEntry // tag -> entry
}

// NewRuleSetManager 创建规则集管理器
func NewRuleSetManager(artifactsDir string) *RuleSetManager {
	return &RuleSetManager{
		artifactsDir: artifactsDir,
		ruleSets:     make(map[string]RuleSetEntry),
	}
}

// ParseGeoRule 解析 geosite:xxx 或 geoip:xxx 格式的规则
// 返回 (类型, 标签, 是否是 geo 规则)
func ParseGeoRule(rule string) (geoType string, tag string, isGeo bool) {
	if strings.HasPrefix(rule, "geosite:") {
		return "geosite", strings.TrimPrefix(rule, "geosite:"), true
	}
	if strings.HasPrefix(rule, "geoip:") {
		return "geoip", strings.TrimPrefix(rule, "geoip:"), true
	}
	return "", "", false
}

// ParseDomainRule 解析域名规则
// 返回 (类型, 值)
// 类型: domain (精确), suffix (后缀), regex (正则), full (完全匹配)
func ParseDomainRule(rule string) (ruleType string, value string) {
	switch {
	case strings.HasPrefix(rule, "domain:"):
		return "suffix", strings.TrimPrefix(rule, "domain:")
	case strings.HasPrefix(rule, "full:"):
		return "domain", strings.TrimPrefix(rule, "full:")
	case strings.HasPrefix(rule, "regexp:"):
		return "regex", strings.TrimPrefix(rule, "regexp:")
	case strings.HasPrefix(rule, "keyword:"):
		return "keyword", strings.TrimPrefix(rule, "keyword:")
	default:
		// 默认按后缀匹配
		return "suffix", rule
	}
}

// AddGeoSite 添加 geosite rule-set
func (m *RuleSetManager) AddGeoSite(name string) string {
	tag := "geosite-" + name
	if _, exists := m.ruleSets[tag]; !exists {
		m.ruleSets[tag] = RuleSetEntry{
			Tag:    tag,
			Type:   "local",
			Format: "binary",
			Path:   filepath.Join(m.artifactsDir, "core", "sing-box", "rule-set", tag+".srs"),
		}
	}
	return tag
}

// AddGeoIP 添加 geoip rule-set
func (m *RuleSetManager) AddGeoIP(name string) string {
	tag := "geoip-" + name
	if _, exists := m.ruleSets[tag]; !exists {
		m.ruleSets[tag] = RuleSetEntry{
			Tag:    tag,
			Type:   "local",
			Format: "binary",
			Path:   filepath.Join(m.artifactsDir, "core", "sing-box", "rule-set", tag+".srs"),
		}
	}
	return tag
}

// GetRuleSets 获取所有 rule-set 声明（用于配置的 route.rule_set）
func (m *RuleSetManager) GetRuleSets() []map[string]interface{} {
	if len(m.ruleSets) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m.ruleSets))
	for k := range m.ruleSets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		rs := m.ruleSets[key]
		entry := map[string]interface{}{
			"tag":    rs.Tag,
			"type":   rs.Type,
			"format": rs.Format,
		}
		if rs.Type == "local" {
			entry["path"] = rs.Path
		} else if rs.Type == "remote" {
			entry["url"] = rs.URL
		}
		result = append(result, entry)
	}
	return result
}

// ConvertRouteMatchRule 将 RouteMatchRule 转换为 sing-box 路由规则
// 返回路由规则和需要的 rule-set 标签
func (m *RuleSetManager) ConvertRouteMatchRule(rule *domain.RouteMatchRule, outbound string) (RoutingRuleEntry, error) {
	entry := RoutingRuleEntry{
		Outbound: outbound,
	}

	if rule == nil {
		return entry, nil
	}

	// 处理域名规则
	for _, d := range rule.Domains {
		geoType, tag, isGeo := ParseGeoRule(d)
		if isGeo {
			if geoType == "geosite" {
				ruleSetTag := m.AddGeoSite(tag)
				entry.RuleSet = append(entry.RuleSet, ruleSetTag)
				continue
			}
			return RoutingRuleEntry{}, fmt.Errorf("geoip rule must be in IPs, not Domains: %s", d)
		}

		// 处理普通域名规则
		ruleType, value := ParseDomainRule(d)
		switch ruleType {
		case "domain":
			entry.Domain = append(entry.Domain, value)
		case "suffix":
			entry.DomainSuffix = append(entry.DomainSuffix, value)
		case "keyword":
			entry.DomainKeyword = append(entry.DomainKeyword, value)
		case "regex":
			entry.DomainRegex = append(entry.DomainRegex, value)
		}
	}

	// 处理 IP 规则
	for _, ip := range rule.IPs {
		geoType, tag, isGeo := ParseGeoRule(ip)
		if isGeo {
			if geoType == "geoip" {
				// 特殊处理 private
				if tag == "private" {
					entry.IPIsPrivate = true
				} else {
					ruleSetTag := m.AddGeoIP(tag)
					entry.RuleSet = append(entry.RuleSet, ruleSetTag)
				}
			}
			continue
		}

		// CIDR 格式
		entry.IPCidr = append(entry.IPCidr, ip)
	}

	// 处理协议
	entry.Protocol = rule.Protocols

	// 处理端口
	entry.Port = rule.Ports

	// 处理进程名
	entry.ProcessName = rule.ProcessNames

	return entry, nil
}

// ToSingBoxRule 将 RoutingRuleEntry 转换为 sing-box 路由规则格式
func (e *RoutingRuleEntry) ToSingBoxRule() map[string]interface{} {
	rule := make(map[string]interface{})

	if len(e.RuleSet) > 0 {
		rule["rule_set"] = e.RuleSet
	}
	if len(e.Domain) > 0 {
		rule["domain"] = e.Domain
	}
	if len(e.DomainSuffix) > 0 {
		rule["domain_suffix"] = e.DomainSuffix
	}
	if len(e.DomainKeyword) > 0 {
		rule["domain_keyword"] = e.DomainKeyword
	}
	if len(e.DomainRegex) > 0 {
		rule["domain_regex"] = e.DomainRegex
	}
	if len(e.IPCidr) > 0 {
		rule["ip_cidr"] = e.IPCidr
	}
	if len(e.Protocol) > 0 {
		rule["protocol"] = e.Protocol
	}
	if len(e.Port) > 0 {
		rule["port"] = e.Port
	}
	if len(e.ProcessName) > 0 {
		rule["process_name"] = e.ProcessName
	}
	if e.IPIsPrivate {
		rule["ip_is_private"] = true
	}
	rule["outbound"] = e.Outbound

	return rule
}

// BuildDefaultRuleSets 构建默认的 rule-set（国内直连、广告拦截）
func (m *RuleSetManager) BuildDefaultRuleSets() {
	// 默认需要的 rule-set
	m.AddGeoSite("cn")
	m.AddGeoSite("category-ads-all")
	m.AddGeoIP("cn")
}

// BuildDefaultRoutingRules 构建默认的路由规则
func (m *RuleSetManager) BuildDefaultRoutingRules(defaultTag string) []map[string]interface{} {
	var rules []map[string]interface{}

	// 1. 广告拦截
	rules = append(rules, map[string]interface{}{
		"rule_set": []string{m.AddGeoSite("category-ads-all")},
		"outbound": "block",
	})

	// 2. 私有 IP 直连
	rules = append(rules, map[string]interface{}{
		"ip_is_private": true,
		"outbound":      "direct",
	})

	// 3. 国内域名直连
	rules = append(rules, map[string]interface{}{
		"rule_set": []string{m.AddGeoSite("cn")},
		"outbound": "direct",
	})

	// 4. 国内 IP 直连
	rules = append(rules, map[string]interface{}{
		"rule_set": []string{m.AddGeoIP("cn")},
		"outbound": "direct",
	})

	return rules
}

// BuildDNSRules 构建 DNS 规则
func (m *RuleSetManager) BuildDNSRules() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"rule_set": []string{m.AddGeoSite("cn")},
			"server":   "dns-local",
		},
	}
}

// 注：sing-box 路由仅允许从 FRouter 编译产物生成，不提供独立的全局分流模型。
