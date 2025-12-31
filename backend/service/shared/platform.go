package shared

import (
	"fmt"
	"runtime"
)

// XrayAssetCandidates 返回 Xray 资源候选列表
func XrayAssetCandidates() ([]string, error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	switch key {
	case "linux/amd64":
		return []string{"Xray-linux-64.zip"}, nil
	case "linux/386":
		return []string{"Xray-linux-32.zip"}, nil
	case "linux/arm64":
		return []string{"Xray-linux-arm64-v8a.zip", "Xray-linux-arm64.zip"}, nil
	case "linux/arm":
		return []string{"Xray-linux-arm32-v7a.zip"}, nil
	case "windows/amd64":
		return []string{"Xray-windows-64.zip"}, nil
	case "windows/386":
		return []string{"Xray-windows-32.zip"}, nil
	case "darwin/amd64":
		return []string{"Xray-macos-64.zip"}, nil
	case "darwin/arm64":
		return []string{"Xray-macos-arm64-v8a.zip", "Xray-macos-arm64.zip"}, nil
	default:
		return nil, fmt.Errorf("unsupported platform %s for xray release asset", key)
	}
}

// SingBoxAssetCandidates 返回 sing-box 资源候选列表
func SingBoxAssetCandidates() ([]string, error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	switch key {
	case "linux/amd64":
		return []string{"sing-box-*-linux-amd64.tar.gz"}, nil
	case "linux/386":
		return []string{"sing-box-*-linux-386.tar.gz"}, nil
	case "linux/arm64":
		return []string{"sing-box-*-linux-arm64.tar.gz"}, nil
	case "linux/arm":
		return []string{"sing-box-*-linux-armv7.tar.gz"}, nil
	case "windows/amd64":
		return []string{"sing-box-*-windows-amd64.zip"}, nil
	case "windows/386":
		return []string{"sing-box-*-windows-386.zip"}, nil
	case "darwin/amd64":
		return []string{"sing-box-*-darwin-amd64.tar.gz"}, nil
	case "darwin/arm64":
		return []string{"sing-box-*-darwin-arm64.tar.gz"}, nil
	default:
		return nil, fmt.Errorf("unsupported platform %s for sing-box release asset", key)
	}
}

// V2RayPluginAssetCandidates 返回 v2ray-plugin 资源候选列表
func V2RayPluginAssetCandidates() ([]string, error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	switch key {
	case "linux/amd64":
		return []string{"v2ray-plugin-linux-amd64-v*.tar.gz"}, nil
	case "linux/386":
		return []string{"v2ray-plugin-linux-386-v*.tar.gz"}, nil
	case "linux/arm64":
		return []string{"v2ray-plugin-linux-arm64-v*.tar.gz"}, nil
	case "linux/arm":
		return []string{"v2ray-plugin-linux-arm-v*.tar.gz"}, nil
	case "windows/amd64":
		return []string{"v2ray-plugin-windows-amd64-v*.tar.gz"}, nil
	case "windows/386":
		return []string{"v2ray-plugin-windows-386-v*.tar.gz"}, nil
	case "darwin/amd64":
		return []string{"v2ray-plugin-darwin-amd64-v*.tar.gz"}, nil
	case "darwin/arm64":
		return []string{"v2ray-plugin-darwin-arm64-v*.tar.gz"}, nil
	default:
		return nil, fmt.Errorf("unsupported platform %s for v2ray-plugin release asset", key)
	}
}

// GetComponentRepo 获取组件的 GitHub 仓库
func GetComponentRepo(kind string) string {
	switch kind {
	case "xray":
		return "XTLS/Xray-core"
	case "singbox":
		return "SagerNet/sing-box"
	case "v2ray-plugin":
		return "shadowsocks/v2ray-plugin"
	default:
		return ""
	}
}

// GetComponentAssetCandidates 获取组件的资源候选列表
func GetComponentAssetCandidates(kind string) ([]string, error) {
	switch kind {
	case "xray":
		return XrayAssetCandidates()
	case "singbox":
		return SingBoxAssetCandidates()
	case "v2ray-plugin":
		return V2RayPluginAssetCandidates()
	default:
		return nil, fmt.Errorf("unknown component kind: %s", kind)
	}
}
