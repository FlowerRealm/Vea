package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"vea/backend/domain"
)

type GeoFiles struct {
	GeoIP   string
	GeoSite string
}

const (
	xrayInboundTag         = "socks-in"
	xrayDirectTag          = "direct"
	xrayBlockTag           = "block"
	xrayDefaultListenAddr  = "127.0.0.1"
	xrayDefaultInboundPort = 38087
)

func buildXrayConfig(nodes []domain.Node, geo GeoFiles, inboundPort int, activeNodeID string) ([]byte, string, error) {
	if len(nodes) == 0 {
		return nil, "", fmt.Errorf("no nodes available to build xray config")
	}

	var (
		outbounds    []map[string]any
		nodeTags     []string
		defaultTag   string
		chosenNodeID string
	)

	for _, node := range nodes {
		outbound, tag, err := buildXrayOutbound(node)
		if err != nil {
			return nil, "", fmt.Errorf("build outbound for %s: %w", node.Name, err)
		}
		outbounds = append(outbounds, outbound)
		nodeTags = append(nodeTags, tag)
		if chosenNodeID == "" {
			chosenNodeID = node.ID
			defaultTag = tag
		}
		if activeNodeID != "" && node.ID == activeNodeID {
			chosenNodeID = node.ID
			defaultTag = tag
		}
	}
	if defaultTag == "" {
		return nil, "", fmt.Errorf("no valid node tags")
	}

	outbounds = append(outbounds,
		map[string]any{"tag": xrayDirectTag, "protocol": "freedom"},
		map[string]any{"tag": xrayBlockTag, "protocol": "blackhole"},
	)

	inbounds := []map[string]any{
		{
			"tag":      xrayInboundTag,
			"listen":   xrayDefaultListenAddr,
			"port":     inboundPort,
			"protocol": "socks",
			"settings": map[string]any{"udp": true, "auth": "noauth"},
			"sniffing": map[string]any{
				"enabled":      true,
				"destOverride": []string{"http", "tls"},
			},
		},
	}

	rules := []map[string]any{
		{"type": "field", "domain": []string{"geosite:category-ads-all"}, "outboundTag": xrayBlockTag},
		{"type": "field", "domain": []string{"geosite:cn", "geosite:private"}, "outboundTag": xrayDirectTag},
		{"type": "field", "ip": []string{"geoip:cn", "geoip:private"}, "outboundTag": xrayDirectTag},
		// catch-all to selected node; rule must have effective fields
		{"type": "field", "network": "tcp,udp", "outboundTag": defaultTag},
	}

	routing := map[string]any{
		"domainStrategy": "AsIs",
		"rules":          rules,
	}

	if geo.GeoIP != "" {
		routing["geoip"] = []map[string]any{{"file": filepath.Clean(geo.GeoIP)}}
	}
	if geo.GeoSite != "" {
		routing["geosite"] = []map[string]any{{"file": filepath.Clean(geo.GeoSite)}}
	}

	config := map[string]any{
		"log":       map[string]any{"loglevel": "debug"},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"routing":   routing,
		"dns": map[string]any{
			"servers": []any{
				"1.1.1.1",
				"8.8.8.8",
			},
		},
	}

	bytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return bytes, chosenNodeID, nil
}

func buildXrayOutbound(node domain.Node) (map[string]any, string, error) {
	tag := fmt.Sprintf("node-%s", shortenID(node.ID))
	outbound := map[string]any{
		"tag":      tag,
		"protocol": strings.ToLower(string(node.Protocol)),
	}

	var (
		settings       map[string]any
		streamSettings map[string]any
	)

	sec := node.Security
	if sec == nil {
		sec = &domain.NodeSecurity{}
	}

	switch node.Protocol {
	case domain.ProtocolVMess:
		settings = map[string]any{
			"vnext": []map[string]any{
				{
					"address": node.Address,
					"port":    node.Port,
					"users": []map[string]any{
						{
							"id":       firstNonEmpty(sec.UUID),
							"alterId":  sec.AlterID,
							"security": firstNonEmpty(sec.Encryption, "auto"),
						},
					},
				},
			},
		}
	case domain.ProtocolVLESS:
		settings = map[string]any{
			"vnext": []map[string]any{
				{
					"address": node.Address,
					"port":    node.Port,
					"users": []map[string]any{
						{
							"id":         firstNonEmpty(sec.UUID),
							"encryption": firstNonEmpty(sec.Encryption, "none"),
							"flow":       sec.Flow,
						},
					},
				},
			},
		}
	case domain.ProtocolTrojan:
		settings = map[string]any{
			"servers": []map[string]any{
				{
					"address":  node.Address,
					"port":     node.Port,
					"password": sec.Password,
					"flow":     sec.Flow,
				},
			},
		}
	case domain.ProtocolShadowsocks:
		server := map[string]any{
			"address":  node.Address,
			"port":     node.Port,
			"method":   sec.Method,
			"password": sec.Password,
			"level":    1,
			"ota":      false,
		}
		if sec.Plugin != "" {
			server["plugin"] = sec.Plugin
		}
		settings = map[string]any{
			"servers": []map[string]any{server},
		}
	default:
		return nil, "", fmt.Errorf("unsupported protocol %s", node.Protocol)
	}

	transport := node.Transport
	if transport != nil || (node.TLS != nil && node.TLS.Enabled) {
		streamSettings = buildXrayStreamSettings(transport, node.TLS)
	}

	// 插件路径已废弃，统一使用内置 tcp/http 头。保留此处为空以简化逻辑。

	if streamSettings != nil {
		outbound["streamSettings"] = streamSettings
	}
	outbound["mux"] = map[string]any{
		"enabled":     false,
		"concurrency": -1,
	}
	outbound["settings"] = settings
	return outbound, tag, nil
}

func buildMeasurementXrayConfig(node domain.Node, inboundPort int) ([]byte, string, error) {
	if node.Security == nil {
		return nil, "", fmt.Errorf("shadowsocks node missing security")
	}

	host := strings.TrimSpace(node.Address)
	path := "/"
	method := "GET"
	version := defaultHTTPObfsHTTPVersion
	acceptEncoding := defaultHTTPObfsAcceptEnc
	connection := defaultHTTPObfsConnection
	userAgent := defaultHTTPObfsUserAgent

	if node.Transport != nil {
		if h := strings.TrimSpace(node.Transport.Host); h != "" {
			host = h
		}
		if p := strings.TrimSpace(node.Transport.Path); p != "" {
			path = p
		}
		if node.Transport.Headers != nil {
			if v := strings.TrimSpace(node.Transport.Headers[shadowsocksHTTPMethodKey]); v != "" {
				method = strings.ToUpper(v)
			}
			if v := strings.TrimSpace(node.Transport.Headers[shadowsocksHTTPVersionKey]); v != "" {
				version = v
			}
			if v := strings.TrimSpace(node.Transport.Headers["Accept-Encoding"]); v != "" {
				acceptEncoding = v
			}
			if v := strings.TrimSpace(node.Transport.Headers["Connection"]); v != "" {
				connection = v
			}
			if v := strings.TrimSpace(node.Transport.Headers["User-Agent"]); v != "" {
				userAgent = v
			}
		}
	}

	if path == "" {
		path = "/"
	}
	if version == "" {
		version = defaultHTTPObfsHTTPVersion
	}
	if method == "" {
		method = "GET"
	}

	requestHeaders := map[string]any{
		"Host":            []string{host},
		"User-Agent":      []string{userAgent},
		"Accept-Encoding": []string{acceptEncoding},
		"Connection":      []string{connection},
		"Pragma":          "no-cache",
	}

	request := map[string]any{
		"version": version,
		"method":  method,
		"path":    []string{path},
		"headers": requestHeaders,
	}

	// 统一使用内置 tcp/http 头，不再使用任何插件
	var stream map[string]any
	stream = map[string]any{
		"network": "tcp",
		"tcpSettings": map[string]any{
			"header": map[string]any{
				"type":    "http",
				"request": request,
			},
		},
	}

	shadowsocksServer := map[string]any{
		"address":  node.Address,
		"port":     node.Port,
		"method":   node.Security.Method,
		"password": node.Security.Password,
		"ota":      false,
		"level":    1,
	}
	// 插件字段移除

	outbound := map[string]any{
		"tag":      fmt.Sprintf("node-%s", shortenID(node.ID)),
		"protocol": "shadowsocks",
		"settings": map[string]any{
			"servers": []map[string]any{shadowsocksServer},
		},
	}
	if stream != nil {
		outbound["streamSettings"] = stream
	}

	config := map[string]any{
		"log": map[string]any{
			"loglevel": "debug",
		},
		"inbounds": []map[string]any{
			{
				"listen":   xrayDefaultListenAddr,
				"port":     inboundPort,
				"protocol": "socks",
				"settings": map[string]any{
					"auth": "noauth",
					"udp":  true,
				},
			},
		},
		"outbounds": []map[string]any{
			outbound,
			{
				"protocol": "freedom",
				"settings": map[string]any{},
				"tag":      xrayDirectTag,
			},
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules": []map[string]any{
				{
					"type":        "field",
					"outboundTag": xrayDirectTag,
					"ip":          []string{"geoip:private"},
				},
			},
		},
	}

	bytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return bytes, node.ID, nil
}

func buildXrayStreamSettings(transport *domain.NodeTransport, tls *domain.NodeTLS) map[string]any {
	if transport == nil {
		transport = &domain.NodeTransport{}
	}
	tlsEnabled := tls != nil && tls.Enabled
	network := strings.ToLower(strings.TrimSpace(transport.Type))
	if network == "" {
		network = "tcp"
	}

	stream := map[string]any{"network": network}

	switch network {
	case "ws":
		ws := map[string]any{}
		if transport.Path != "" {
			ws["path"] = transport.Path
		}
		if host := transport.Host; host != "" {
			headers := map[string]any{"Host": host}
			ws["headers"] = headers
		}
		stream["wsSettings"] = ws
	case "grpc":
		grpc := map[string]any{}
		if transport.ServiceName != "" {
			grpc["serviceName"] = transport.ServiceName
		}
		stream["grpcSettings"] = grpc
	case "http":
		http := map[string]any{}
		if transport.Host != "" {
			http["host"] = []string{transport.Host}
		}
		if transport.Path != "" {
			http["path"] = transport.Path
		}
		stream["httpSettings"] = http
	case "tcp":
		path := strings.TrimSpace(transport.Path)
		if path == "" {
			path = "/"
		}
		if transport.Host != "" || len(transport.Headers) > 0 || path != "" {
			request := map[string]any{}
			headerMap := map[string]any{}

			if transport.Host != "" {
				headerMap["Host"] = []string{transport.Host}
			}
			request["path"] = []string{path}

			method := strings.TrimSpace(transport.Headers[shadowsocksHTTPMethodKey])
			if method == "" {
				method = "GET"
			}
			version := strings.TrimSpace(transport.Headers[shadowsocksHTTPVersionKey])
			if version == "" {
				version = "1.1"
			}
			request["method"] = method
			request["version"] = version

			for k, v := range transport.Headers {
				key := strings.TrimSpace(k)
				if key == "" || key == shadowsocksHTTPMethodKey || key == shadowsocksHTTPVersionKey {
					continue
				}
				value := strings.TrimSpace(v)
				if value == "" {
					continue
				}
				if strings.EqualFold(key, "Pragma") {
					// 与常见示例一致：Pragma 使用字符串而非数组
					headerMap[key] = value
				} else {
					headerMap[key] = []string{value}
				}
			}
			if len(headerMap) > 0 {
				request["headers"] = headerMap
			}
			header := map[string]any{
				"type":    "http",
				"request": request,
			}
			stream["tcpSettings"] = map[string]any{
				"header": header,
			}
		}
		// 当未启用 TLS 时不显式写 security，默认即为 none，避免冗余字段。
	}

	if tlsEnabled {
		securityType := strings.ToLower(strings.TrimSpace(tls.Type))
		switch {
		case strings.Contains(securityType, "reality"):
			stream["security"] = "reality"
			settings := map[string]any{}
			if tls.ServerName != "" {
				settings["serverName"] = tls.ServerName
			}
			if tls.RealityPublicKey != "" {
				settings["publicKey"] = tls.RealityPublicKey
			}
			if tls.RealityShortID != "" {
				settings["shortId"] = tls.RealityShortID
			}
			if tls.Fingerprint != "" {
				settings["fingerprint"] = tls.Fingerprint
			}
			if len(settings) > 0 {
				stream["realitySettings"] = settings
			}
		case strings.Contains(securityType, "xtls"):
			stream["security"] = "xtls"
			settings := map[string]any{
				"allowInsecure": tls.Insecure,
			}
			if tls.ServerName != "" {
				settings["serverName"] = tls.ServerName
			}
			if len(tls.ALPN) > 0 {
				settings["alpn"] = tls.ALPN
			}
			if tls.Fingerprint != "" {
				settings["fingerprint"] = tls.Fingerprint
			}
			stream["xtlsSettings"] = settings
		default:
			stream["security"] = "tls"
			settings := map[string]any{
				"allowInsecure": tls.Insecure,
			}
			if tls.ServerName != "" {
				settings["serverName"] = tls.ServerName
			}
			if len(tls.ALPN) > 0 {
				settings["alpn"] = tls.ALPN
			}
			if tls.Fingerprint != "" {
				settings["fingerprint"] = tls.Fingerprint
			}
			stream["tlsSettings"] = settings
		}
	}

	return stream
}

func shortenID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
