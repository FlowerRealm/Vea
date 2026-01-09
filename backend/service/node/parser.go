package node

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"vea/backend/domain"
	"vea/backend/service/shared"
)

// ParseShareLink 解析分享链接
func ParseShareLink(link string) (domain.Node, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return domain.Node{}, ErrInvalidShareLink
	}

	// 根据协议前缀选择解析器
	switch {
	case strings.HasPrefix(link, "vless://"):
		return parseVLESS(link)
	case strings.HasPrefix(link, "vmess://"):
		return parseVMess(link)
	case strings.HasPrefix(link, "trojan://"):
		return parseTrojan(link)
	case strings.HasPrefix(link, "ss://"):
		return parseShadowsocks(link)
	default:
		return domain.Node{}, ErrInvalidShareLink
	}
}

// parseVLESS 解析 VLESS 链接
func parseVLESS(link string) (domain.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return domain.Node{}, err
	}

	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}

	node := domain.Node{
		Name:     u.Fragment,
		Address:  u.Hostname(),
		Port:     port,
		Protocol: domain.ProtocolVLESS,
		Security: &domain.NodeSecurity{
			UUID: u.User.Username(),
			Flow: u.Query().Get("flow"),
		},
	}

	// 解析传输层
	transportType := u.Query().Get("type")
	if transportType == "" {
		transportType = "tcp"
	}
	node.Transport = &domain.NodeTransport{
		Type:        transportType,
		Host:        u.Query().Get("host"),
		Path:        u.Query().Get("path"),
		ServiceName: u.Query().Get("serviceName"),
	}

	// 解析 TLS
	security := u.Query().Get("security")
	if security == "tls" || security == "reality" {
		node.TLS = &domain.NodeTLS{
			Enabled:          true,
			Type:             security,
			ServerName:       u.Query().Get("sni"),
			Fingerprint:      u.Query().Get("fp"),
			RealityPublicKey: u.Query().Get("pbk"),
			RealityShortID:   u.Query().Get("sid"),
		}
		if alpn := u.Query().Get("alpn"); alpn != "" {
			node.TLS.ALPN = strings.Split(alpn, ",")
		}
	}

	if node.Name == "" {
		node.Name = node.Address
	}

	return node, nil
}

// parseVMess 解析 VMess 链接
func parseVMess(link string) (domain.Node, error) {
	// VMess 链接格式: vmess://base64(json)
	encoded := strings.TrimPrefix(link, "vmess://")

	// 灵活解码 base64
	decoded, err := decodeBase64Flexible(encoded)
	if err != nil {
		return domain.Node{}, errors.New("invalid vmess base64")
	}

	// 解析 JSON
	var vmessConfig struct {
		V    interface{} `json:"v"`    // 版本，可能是 string 或 int
		PS   string      `json:"ps"`   // 节点名称
		Add  string      `json:"add"`  // 服务器地址
		Port interface{} `json:"port"` // 端口，可能是 string 或 int
		ID   string      `json:"id"`   // UUID
		Aid  interface{} `json:"aid"`  // AlterID，可能是 string 或 int
		Scy  string      `json:"scy"`  // 加密方式
		Net  string      `json:"net"`  // 传输协议
		Type string      `json:"type"` // 伪装类型
		Host string      `json:"host"` // 伪装域名
		Path string      `json:"path"` // 路径
		TLS  string      `json:"tls"`  // TLS
		SNI  string      `json:"sni"`  // SNI
		ALPN string      `json:"alpn"` // ALPN
		FP   string      `json:"fp"`   // 指纹
	}

	if err := json.Unmarshal(decoded, &vmessConfig); err != nil {
		return domain.Node{}, errors.New("invalid vmess json: " + err.Error())
	}

	// 解析端口
	var port int
	switch v := vmessConfig.Port.(type) {
	case string:
		port, _ = strconv.Atoi(v)
	case float64:
		port = int(v)
	case int:
		port = v
	}
	if port == 0 {
		port = 443
	}

	// 解析 AlterID
	var alterID int
	switch v := vmessConfig.Aid.(type) {
	case string:
		alterID, _ = strconv.Atoi(v)
	case float64:
		alterID = int(v)
	case int:
		alterID = v
	}

	// 确定加密方式
	security := vmessConfig.Scy
	if security == "" {
		security = "auto"
	}

	node := domain.Node{
		Name:     vmessConfig.PS,
		Address:  vmessConfig.Add,
		Port:     port,
		Protocol: domain.ProtocolVMess,
		Security: &domain.NodeSecurity{
			UUID:    vmessConfig.ID,
			AlterID: alterID,
			Method:  security,
		},
	}

	// 解析传输层
	transportType := vmessConfig.Net
	if transportType == "" {
		transportType = "tcp"
	}
	node.Transport = &domain.NodeTransport{
		Type: transportType,
		Host: vmessConfig.Host,
		Path: vmessConfig.Path,
	}

	// 处理伪装类型
	if vmessConfig.Type != "" && vmessConfig.Type != "none" {
		node.Transport.HeaderType = vmessConfig.Type
	}

	// 解析 TLS
	if vmessConfig.TLS == "tls" {
		sni := vmessConfig.SNI
		if sni == "" {
			sni = vmessConfig.Host
		}
		node.TLS = &domain.NodeTLS{
			Enabled:     true,
			Type:        "tls",
			ServerName:  sni,
			Fingerprint: vmessConfig.FP,
		}
		if vmessConfig.ALPN != "" {
			node.TLS.ALPN = strings.Split(vmessConfig.ALPN, ",")
		}
	}

	if node.Name == "" {
		node.Name = node.Address
	}

	return node, nil
}

// parseTrojan 解析 Trojan 链接
func parseTrojan(link string) (domain.Node, error) {
	u, err := url.Parse(link)
	if err != nil {
		return domain.Node{}, err
	}

	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}

	node := domain.Node{
		Name:     u.Fragment,
		Address:  u.Hostname(),
		Port:     port,
		Protocol: domain.ProtocolTrojan,
		Security: &domain.NodeSecurity{
			Password: u.User.Username(),
		},
		TLS: &domain.NodeTLS{
			Enabled:    true,
			ServerName: u.Query().Get("sni"),
		},
	}

	// 解析传输层
	transportType := u.Query().Get("type")
	if transportType == "" {
		transportType = "tcp"
	}
	node.Transport = &domain.NodeTransport{
		Type: transportType,
		Host: u.Query().Get("host"),
		Path: u.Query().Get("path"),
	}

	if node.Name == "" {
		node.Name = node.Address
	}

	return node, nil
}

// parseShadowsocks 解析 Shadowsocks 链接
func parseShadowsocks(link string) (domain.Node, error) {
	// SS 链接格式:
	// 1. SIP002: ss://base64(method:password)@host:port/?plugin=...#name
	link = strings.TrimPrefix(link, "ss://")

	// 分离名称
	var name string
	if idx := strings.LastIndex(link, "#"); idx != -1 {
		name, _ = url.QueryUnescape(link[idx+1:])
		link = link[:idx]
	}

	// 格式1: SIP002 - base64(method:password)@host:port/?plugin=...
	if strings.Contains(link, "@") {
		parts := strings.SplitN(link, "@", 2)
		if len(parts) == 2 {
			// 解码 method:password
			decoded, err := decodeBase64Flexible(parts[0])
			if err == nil {
				methodPass := strings.SplitN(string(decoded), ":", 2)
				if len(methodPass) == 2 {
					// 解析 host:port/?plugin=...
					hostPortQuery := parts[1]

					// 分离查询参数
					var queryStr string
					if qIdx := strings.Index(hostPortQuery, "?"); qIdx != -1 {
						queryStr = hostPortQuery[qIdx+1:]
						hostPortQuery = hostPortQuery[:qIdx]
					}
					// 分离路径（有些链接格式为 host:port/path?...）
					if sIdx := strings.Index(hostPortQuery, "/"); sIdx != -1 {
						hostPortQuery = hostPortQuery[:sIdx]
					}

					host, portStr, err := splitHostPort(hostPortQuery)
					if err != nil {
						return domain.Node{}, fmt.Errorf("invalid host:port: %w", err)
					}
					if portStr == "" {
						return domain.Node{}, errors.New("missing port in ss link")
					}
					port, err := strconv.Atoi(portStr)
					if err != nil || port <= 0 || port > 65535 {
						return domain.Node{}, fmt.Errorf("invalid port: %s", portStr)
					}

					node := domain.Node{
						Name:     name,
						Address:  host,
						Port:     port,
						Protocol: domain.ProtocolShadowsocks,
						Security: &domain.NodeSecurity{
							Method:   methodPass[0],
							Password: methodPass[1],
						},
					}

					// 解析 plugin 参数
					if queryStr != "" {
						params, _ := url.ParseQuery(queryStr)
						if plugin := params.Get("plugin"); plugin != "" {
							// plugin 格式: obfs-local;obfs=http;obfs-host=xxx
							pluginParts := strings.SplitN(plugin, ";", 2)
							pluginName := pluginParts[0]
							pluginOpts := ""
							if len(pluginParts) > 1 {
								pluginOpts = pluginParts[1]
							}
							pluginName, pluginOpts = shared.NormalizeShadowsocksPluginAlias(pluginName, pluginOpts)
							node.Security.Plugin = pluginName
							node.Security.PluginOpts = pluginOpts
						}
					}

					if node.Name == "" {
						node.Name = node.Address
					}
					return node, nil
				}
			}
		}
	}

	return domain.Node{}, errors.New("unsupported ss link format")
}

// splitHostPort 分离 host 和 port
func splitHostPort(hostPort string) (string, string, error) {
	// 处理 IPv6 地址
	if strings.HasPrefix(hostPort, "[") {
		end := strings.Index(hostPort, "]")
		if end == -1 {
			return "", "", errors.New("invalid IPv6 address")
		}
		host := hostPort[1:end]
		if len(hostPort) > end+2 && hostPort[end+1] == ':' {
			return host, hostPort[end+2:], nil
		}
		return host, "", nil
	}

	// 普通 host:port
	idx := strings.LastIndex(hostPort, ":")
	if idx == -1 {
		return hostPort, "", nil
	}
	return hostPort[:idx], hostPort[idx+1:], nil
}

// ParseMultipleLinks 解析多个分享链接
// 支持 base64 编码的订阅内容和纯文本链接列表
func ParseMultipleLinks(links string) ([]domain.Node, []error) {
	var errs []error

	text := strings.TrimSpace(links)
	if text == "" {
		return nil, nil
	}

	// 尝试解码 base64（订阅内容通常是 base64 编码的）
	candidates := []string{}

	// 清理换行符后尝试 base64 解码
	cleaned := strings.ReplaceAll(strings.ReplaceAll(text, "\r", ""), "\n", "")
	if decoded, err := decodeBase64Flexible(cleaned); err == nil {
		decodedText := strings.TrimSpace(string(decoded))
		if decodedText != "" && isLikelyShareLinks(decodedText) {
			candidates = append(candidates, decodedText)
		}
	}
	// 原始文本作为候选
	candidates = append(candidates, text)

	// 遍历候选内容，解析节点
	for _, candidate := range candidates {
		var nodes []domain.Node
		lines := strings.Split(candidate, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			// 跳过非分享链接
			if !isShareLink(line) {
				continue
			}

			node, err := ParseShareLink(line)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			nodes = append(nodes, node)
		}
		// 如果解析出节点，使用这个候选
		if len(nodes) > 0 {
			filtered := filterSubscriptionNodes(nodes)
			if len(filtered) > 0 {
				return filtered, errs
			}
		}
	}

	return nil, errs
}

// decodeBase64Flexible 灵活解码 base64（自动补齐 padding）
func decodeBase64Flexible(value string) ([]byte, error) {
	switch len(value) % 4 {
	case 2:
		value += "=="
	case 3:
		value += "="
	}
	if data, err := base64.StdEncoding.DecodeString(value); err == nil {
		return data, nil
	}
	return base64.URLEncoding.DecodeString(value)
}

// isShareLink 检查是否是分享链接
func isShareLink(line string) bool {
	return strings.HasPrefix(line, "vmess://") ||
		strings.HasPrefix(line, "vless://") ||
		strings.HasPrefix(line, "trojan://") ||
		strings.HasPrefix(line, "ss://")
}

// isLikelyShareLinks 检查解码后的内容是否像分享链接列表
func isLikelyShareLinks(text string) bool {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if isShareLink(line) {
			return true
		}
	}
	return false
}

var subscriptionInfoKeywords = []string{
	"剩余", "流量", "到期", "过期", "有效期",
	"升级", "版本", "客户端", "官网", "教程",
	"traffic", "expire", "expired", "upgrade", "version",
}

func filterSubscriptionNodes(nodes []domain.Node) []domain.Node {
	if len(nodes) == 0 {
		return nodes
	}
	out := make([]domain.Node, 0, len(nodes))
	for _, n := range nodes {
		if isLikelySubscriptionInfoNode(n) {
			continue
		}
		out = append(out, n)
	}
	return out
}

func isLikelySubscriptionInfoNode(node domain.Node) bool {
	addr := strings.ToLower(strings.TrimSpace(node.Address))
	name := strings.ToLower(strings.TrimSpace(node.Name))
	if addr == "" || name == "" {
		return false
	}

	// 订阅“提示节点”通常指向本地回环端口（如 127.0.0.1:1080）。
	isLoopback := addr == "127.0.0.1" || addr == "localhost" || addr == "0.0.0.0"
	if !isLoopback {
		return false
	}
	if node.Port != 1080 && node.Port != 0 {
		return false
	}

	for _, kw := range subscriptionInfoKeywords {
		if strings.Contains(name, kw) {
			return true
		}
	}
	return false
}
