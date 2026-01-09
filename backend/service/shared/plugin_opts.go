package shared

import "strings"

// ParsePluginOptsString parses a plugin options string like "k=v;flag" into a map.
//
// Notes:
// - Pairs are split by ';'
// - Key/value pairs use the first '='
// - Keys and values are trimmed; empty key/value pairs are dropped
// - Bare keys (without '=') are preserved with an empty value
func ParsePluginOptsString(opts string) map[string]string {
	opts = strings.TrimSpace(opts)
	if opts == "" {
		return nil
	}

	out := make(map[string]string)
	for _, part := range strings.Split(opts, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if k, v, ok := strings.Cut(part, "="); ok {
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if k == "" || v == "" {
				continue
			}
			out[k] = v
			continue
		}
		out[part] = ""
	}

	return out
}

// NormalizeShadowsocksPluginAlias normalizes Shadowsocks plugin naming and options.
//
// Currently supported:
//   - plugin=obfs (Clash/Mihomo alias) -> plugin=obfs-local
//   - For obfs-local, normalize opts keys to "obfs/obfs-host/obfs-uri", accepting
//     aliases "mode/host/path" and keeping order stable.
func NormalizeShadowsocksPluginAlias(plugin, opts string) (string, string) {
	plugin = strings.TrimSpace(plugin)
	opts = strings.TrimSpace(opts)
	if plugin == "" {
		return "", ""
	}

	// Clash/Mihomo uses plugin=obfs but it is actually simple-obfs's obfs-local.
	if strings.EqualFold(plugin, "obfs") || strings.EqualFold(plugin, "obfs-local") {
		plugin = "obfs-local"
		normalized, ok := normalizeObfsLocalOpts(opts)
		if ok {
			return plugin, normalized
		}
		return plugin, opts
	}

	return plugin, opts
}

func normalizeObfsLocalOpts(opts string) (string, bool) {
	opts = strings.TrimSpace(opts)
	if opts == "" {
		return "", false
	}

	kv := ParsePluginOptsString(opts)
	if len(kv) == 0 {
		return "", false
	}

	obfsMode := firstNonEmpty(kv["obfs"], kv["mode"])
	obfsHost := firstNonEmpty(kv["obfs-host"], kv["host"])
	obfsURI := firstNonEmpty(kv["obfs-uri"], kv["path"])
	if obfsMode == "" && obfsHost == "" && obfsURI == "" {
		return "", false
	}

	parts := make([]string, 0, 3)
	if obfsMode != "" {
		parts = append(parts, "obfs="+obfsMode)
	}
	if obfsHost != "" {
		parts = append(parts, "obfs-host="+obfsHost)
	}
	if obfsURI != "" {
		parts = append(parts, "obfs-uri="+obfsURI)
	}
	return strings.Join(parts, ";"), true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
