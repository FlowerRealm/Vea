// 核心组件版本探测：从二进制路径/输出中尽力解析版本号，用于 UI 展示与状态回填。
package shared

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var (
	semverRe = regexp.MustCompile(`(?i)\bv?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?\b`)

	singBoxVersionRe = regexp.MustCompile(`(?i)\bsing-box\b[^\n]*?\b(v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)\b`)
	clashVersionRe   = regexp.MustCompile(`(?i)\b(mihomo|clash)\b[^\n]*?\b(v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)\b`)
)

func DetectCoreBinaryVersion(kind, binaryPath string) (string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		return "", errors.New("binary path is empty")
	}

	if v := extractVersionFromPath(binaryPath); v != "" {
		return normalizeVersionTag(v), nil
	}

	var lastErr error
	for _, args := range coreVersionArgs(kind) {
		out, err := runVersionProbe(binaryPath, args)
		if err != nil {
			lastErr = err
		}

		if v := parseCoreBinaryVersion(kind, out); v != "" {
			return normalizeVersionTag(v), nil
		}

		if errors.Is(err, context.DeadlineExceeded) {
			break
		}
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("version not found")
}

func coreVersionArgs(kind string) [][]string {
	switch kind {
	case "singbox", "sing-box":
		return [][]string{{"version"}, {"--version"}, {"-v"}, {"-V"}}
	case "clash", "mihomo":
		return [][]string{{"-v"}, {"--version"}, {"version"}, {"-V"}}
	default:
		return [][]string{{"--version"}, {"version"}, {"-v"}, {"-V"}}
	}
}

func runVersionProbe(binaryPath string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	out, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return string(out), context.DeadlineExceeded
	}
	return string(out), err
}

func parseCoreBinaryVersion(kind, output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}

	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "singbox", "sing-box":
		if m := singBoxVersionRe.FindStringSubmatch(output); len(m) == 2 {
			return m[1]
		}
	case "clash", "mihomo":
		if m := clashVersionRe.FindStringSubmatch(output); len(m) >= 3 {
			return m[2]
		}
	}

	// fallback: scan semver-like tokens, skip go version (e.g. go1.22.0)
	indexes := semverRe.FindAllStringIndex(output, -1)
	for _, idx := range indexes {
		if len(idx) != 2 {
			continue
		}
		start, end := idx[0], idx[1]
		if start >= 2 && strings.EqualFold(output[start-2:start], "go") {
			continue
		}
		return output[start:end]
	}

	return ""
}

func extractVersionFromPath(path string) string {
	path = strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	if path == "" {
		return ""
	}
	return semverRe.FindString(path)
}

func normalizeVersionTag(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if strings.HasPrefix(version, "v") || strings.HasPrefix(version, "V") {
		return "v" + strings.TrimSpace(version[1:])
	}
	return fmt.Sprintf("v%s", version)
}
