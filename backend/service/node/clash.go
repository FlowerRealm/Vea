package node

import (
	"fmt"
	"strconv"
	"strings"

	"vea/backend/domain"

	"gopkg.in/yaml.v3"
)

func parseClashYAMLNodes(text string) ([]domain.Node, []error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(text), &raw); err != nil {
		return nil, []error{fmt.Errorf("invalid clash yaml: %w", err)}
	}
	proxies := pickClashProxies(raw)
	if len(proxies) == 0 {
		return nil, nil
	}

	nodes := make([]domain.Node, 0, len(proxies))
	errs := make([]error, 0)
	for _, p := range proxies {
		node, ok, err := parseClashProxy(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !ok {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes, errs
}

func pickClashProxies(raw map[string]interface{}) []map[string]interface{} {
	if raw == nil {
		return nil
	}
	norm := normalizeStringMap(raw)
	for _, key := range []string{"proxies", "proxy"} {
		if v, ok := norm[key]; ok {
			return asMapSlice(v)
		}
	}
	return nil
}

func parseClashProxy(m map[string]interface{}) (domain.Node, bool, error) {
	if m == nil {
		return domain.Node{}, false, nil
	}
	name := strings.TrimSpace(getString(m, "name"))
	proxyType := strings.ToLower(strings.TrimSpace(getString(m, "type")))
	server := strings.TrimSpace(getString(m, "server"))
	port := getInt(m, "port")
	if name == "" || proxyType == "" || server == "" || port <= 0 {
		return domain.Node{}, false, fmt.Errorf("invalid clash proxy entry: name/type/server/port required")
	}

	switch proxyType {
	case "vless":
		return parseClashVLESS(name, server, port, m)
	case "vmess":
		return parseClashVMess(name, server, port, m)
	case "trojan":
		return parseClashTrojan(name, server, port, m)
	case "ss", "shadowsocks":
		return parseClashShadowsocks(name, server, port, m)
	default:
		return domain.Node{}, false, nil
	}
}

func parseClashVLESS(name, server string, port int, m map[string]interface{}) (domain.Node, bool, error) {
	uuid := strings.TrimSpace(getString(m, "uuid"))
	if uuid == "" {
		return domain.Node{}, false, fmt.Errorf("clash vless proxy %q: missing uuid", name)
	}
	node := domain.Node{
		Name:     name,
		Address:  server,
		Port:     port,
		Protocol: domain.ProtocolVLESS,
		Security: &domain.NodeSecurity{
			UUID: uuid,
			Flow: strings.TrimSpace(getString(m, "flow")),
		},
	}
	applyClashTransport(&node, m)
	applyClashTLS(&node, m)
	applyClashReality(&node, m)
	return node, true, nil
}

func parseClashVMess(name, server string, port int, m map[string]interface{}) (domain.Node, bool, error) {
	uuid := strings.TrimSpace(getString(m, "uuid"))
	if uuid == "" {
		return domain.Node{}, false, fmt.Errorf("clash vmess proxy %q: missing uuid", name)
	}
	alterID := getInt(m, "alterId", "alter-id", "aid")
	security := strings.TrimSpace(getString(m, "cipher"))
	if security == "" {
		security = "auto"
	}
	node := domain.Node{
		Name:     name,
		Address:  server,
		Port:     port,
		Protocol: domain.ProtocolVMess,
		Security: &domain.NodeSecurity{
			UUID:    uuid,
			AlterID: alterID,
			Method:  security,
		},
	}
	applyClashTransport(&node, m)
	applyClashTLS(&node, m)
	return node, true, nil
}

func parseClashTrojan(name, server string, port int, m map[string]interface{}) (domain.Node, bool, error) {
	password := strings.TrimSpace(getString(m, "password"))
	if password == "" {
		return domain.Node{}, false, fmt.Errorf("clash trojan proxy %q: missing password", name)
	}
	node := domain.Node{
		Name:     name,
		Address:  server,
		Port:     port,
		Protocol: domain.ProtocolTrojan,
		Security: &domain.NodeSecurity{
			Password: password,
		},
	}
	applyClashTransport(&node, m)
	applyClashTLS(&node, m)
	return node, true, nil
}

func parseClashShadowsocks(name, server string, port int, m map[string]interface{}) (domain.Node, bool, error) {
	password := strings.TrimSpace(getString(m, "password"))
	method := strings.TrimSpace(getString(m, "cipher"))
	if password == "" || method == "" {
		return domain.Node{}, false, fmt.Errorf("clash ss proxy %q: missing cipher/password", name)
	}
	node := domain.Node{
		Name:     name,
		Address:  server,
		Port:     port,
		Protocol: domain.ProtocolShadowsocks,
		Security: &domain.NodeSecurity{
			Method:   method,
			Password: password,
		},
	}
	applyClashTransport(&node, m)
	applyClashTLS(&node, m)
	return node, true, nil
}

func applyClashTransport(node *domain.Node, m map[string]interface{}) {
	if node == nil {
		return
	}
	network := strings.ToLower(strings.TrimSpace(getString(m, "network")))
	if network == "" {
		network = "tcp"
	}

	t := &domain.NodeTransport{
		Type: network,
	}
	switch network {
	case "ws":
		wsOpts := getMap(m, "ws-opts", "wsOpts")
		t.Path = strings.TrimSpace(getString(wsOpts, "path"))
		headers := getMap(wsOpts, "headers")
		t.Host = strings.TrimSpace(getString(headers, "Host", "host"))
	case "grpc":
		grpcOpts := getMap(m, "grpc-opts", "grpcOpts")
		t.ServiceName = strings.TrimSpace(getString(grpcOpts, "grpc-service-name", "serviceName", "service-name"))
	}
	node.Transport = t
}

func applyClashTLS(node *domain.Node, m map[string]interface{}) {
	if node == nil {
		return
	}
	if !getBool(m, "tls") {
		return
	}

	serverName := strings.TrimSpace(getString(m, "servername", "serverName", "sni"))
	node.TLS = &domain.NodeTLS{
		Enabled:    true,
		Type:       "tls",
		ServerName: serverName,
		Insecure:   getBool(m, "skip-cert-verify", "skipCertVerify", "insecure"),
	}

	if alpnRaw := strings.TrimSpace(getString(m, "alpn")); alpnRaw != "" {
		node.TLS.ALPN = splitCommaList(alpnRaw)
	}
	if fp := strings.TrimSpace(getString(m, "fingerprint", "fp")); fp != "" {
		node.TLS.Fingerprint = fp
	}
}

func applyClashReality(node *domain.Node, m map[string]interface{}) {
	if node == nil {
		return
	}
	realityOpts := getMap(m, "reality-opts", "realityOpts")
	if len(realityOpts) == 0 {
		return
	}
	if node.TLS == nil {
		node.TLS = &domain.NodeTLS{Enabled: true}
	}
	node.TLS.Type = "reality"
	if pk := strings.TrimSpace(getString(realityOpts, "public-key", "publicKey", "pbk")); pk != "" {
		node.TLS.RealityPublicKey = pk
	}
	if sid := strings.TrimSpace(getString(realityOpts, "short-id", "shortId", "sid")); sid != "" {
		node.TLS.RealityShortID = sid
	}
}

func looksLikeClashYAML(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "proxies:") ||
		strings.Contains(lower, "proxy-groups:") ||
		strings.Contains(lower, "proxy group:") ||
		strings.Contains(lower, "\nrules:") ||
		strings.Contains(lower, "\nproxy:")
}

func normalizeStringMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[normalizeKey(k)] = v
	}
	return out
}

func normalizeKey(k string) string {
	k = strings.ToLower(k)
	k = strings.ReplaceAll(k, "-", "")
	k = strings.ReplaceAll(k, "_", "")
	k = strings.ReplaceAll(k, " ", "")
	return k
}

func asMapSlice(v interface{}) []map[string]interface{} {
	list, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if ok {
			out = append(out, m)
			continue
		}
		if m2, ok := item.(map[interface{}]interface{}); ok {
			out = append(out, convertMap(m2))
		}
	}
	return out
}

func convertMap(in map[interface{}]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		ks, ok := k.(string)
		if !ok {
			continue
		}
		switch vv := v.(type) {
		case map[interface{}]interface{}:
			out[ks] = convertMap(vv)
		case []interface{}:
			out[ks] = convertSlice(vv)
		default:
			out[ks] = v
		}
	}
	return out
}

func convertSlice(in []interface{}) []interface{} {
	out := make([]interface{}, 0, len(in))
	for _, item := range in {
		switch vv := item.(type) {
		case map[interface{}]interface{}:
			out = append(out, convertMap(vv))
		case []interface{}:
			out = append(out, convertSlice(vv))
		default:
			out = append(out, item)
		}
	}
	return out
}

func getString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprint(v)
		}
		// 兼容 yaml.v2 风格 map[interface{}]interface{} 的递归转换入口
		if v, ok := m[normalizeKey(k)]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func getInt(m map[string]interface{}, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if i, ok := toInt(v); ok {
				return i
			}
		}
		if v, ok := m[normalizeKey(k)]; ok {
			if i, ok := toInt(v); ok {
				return i
			}
		}
	}
	return 0
}

func toInt(v interface{}) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case float32:
		return int(x), true
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(x))
		return i, err == nil
	default:
		return 0, false
	}
}

func getBool(m map[string]interface{}, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return toBool(v)
		}
		if v, ok := m[normalizeKey(k)]; ok {
			return toBool(v)
		}
	}
	return false
}

func toBool(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		b, _ := strconv.ParseBool(strings.TrimSpace(x))
		return b
	default:
		return false
	}
}

func getMap(m map[string]interface{}, keys ...string) map[string]interface{} {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return asStringMap(v)
		}
		if v, ok := m[normalizeKey(k)]; ok {
			return asStringMap(v)
		}
	}
	return nil
}

func asStringMap(v interface{}) map[string]interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		return x
	case map[interface{}]interface{}:
		return convertMap(x)
	default:
		return nil
	}
}

func splitCommaList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
