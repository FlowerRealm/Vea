package service

import (
	"fmt"
	"os"
	"path/filepath"

	"vea/backend/domain"
)

// SelectEngine 自动选择最佳内核引擎
// 规则优先级：
// 1. TUN 模式强制 sing-box
// 2. Hysteria2/TUIC 强制 sing-box
// 3. 用户偏好（如果兼容）
// 4. 默认 Xray（通用协议）
// 5. 回退到 sing-box
func (s *Service) SelectEngine(profile domain.ProxyProfile, node domain.Node) (domain.CoreEngineKind, error) {
	// 规则 1: TUN 模式强制 sing-box
	if profile.InboundMode == domain.InboundTUN {
		if !s.isEngineInstalled(domain.EngineSingBox) {
			return "", fmt.Errorf("TUN mode requires sing-box, but it's not installed")
		}
		return domain.EngineSingBox, nil
	}

	// 规则 2: Hysteria2/TUIC 强制 sing-box
	if node.Protocol == "hysteria2" || node.Protocol == "tuic" {
		if !s.isEngineInstalled(domain.EngineSingBox) {
			return "", fmt.Errorf("protocol %s requires sing-box", node.Protocol)
		}
		return domain.EngineSingBox, nil
	}

	// 规则 3: 用户偏好（如果兼容且不是 auto）
	if profile.PreferredEngine != domain.EngineAuto && profile.PreferredEngine != "" {
		if s.engineSupportsNode(profile.PreferredEngine, node) {
			if s.isEngineInstalled(profile.PreferredEngine) {
				return profile.PreferredEngine, nil
			}
		}
	}

	// 规则 4: 默认 Xray（通用协议）
	if s.isEngineInstalled(domain.EngineXray) {
		if s.engineSupportsNode(domain.EngineXray, node) {
			return domain.EngineXray, nil
		}
	}

	// 规则 5: 回退到 sing-box
	if s.isEngineInstalled(domain.EngineSingBox) {
		return domain.EngineSingBox, nil
	}

	return "", fmt.Errorf("no compatible engine installed for protocol %s", node.Protocol)
}

// engineSupportsNode 检查引擎是否支持特定节点协议
func (s *Service) engineSupportsNode(engine domain.CoreEngineKind, node domain.Node) bool {
	switch engine {
	case domain.EngineXray:
		// Xray 支持的协议
		return node.Protocol == domain.ProtocolVLESS ||
			node.Protocol == domain.ProtocolVMess ||
			node.Protocol == domain.ProtocolTrojan ||
			node.Protocol == domain.ProtocolShadowsocks

	case domain.EngineSingBox:
		// sing-box 支持所有协议（包括 Xray 的 + Hysteria2/TUIC）
		return true

	default:
		return false
	}
}

// isEngineInstalled 检查引擎是否已安装
func (s *Service) isEngineInstalled(kind domain.CoreEngineKind) bool {
	components := s.ListComponents()
	for _, comp := range components {
		// 匹配 ComponentKind 和 CoreEngineKind
		if (kind == domain.EngineXray && comp.Kind == domain.ComponentXray) ||
			(kind == domain.EngineSingBox && comp.Kind == domain.ComponentSingBox) {
			// 检查是否有安装目录和二进制文件
			if comp.InstallDir != "" && comp.LastInstalledAt.Unix() > 0 {
				return true
			}
		}
	}
	return false
}

// getEngineBinaryPath 获取引擎二进制文件路径
func (s *Service) getEngineBinaryPath(kind domain.CoreEngineKind) (string, error) {
	components := s.ListComponents()
	for _, comp := range components {
		if (kind == domain.EngineXray && comp.Kind == domain.ComponentXray) ||
			(kind == domain.EngineSingBox && comp.Kind == domain.ComponentSingBox) {
			if comp.InstallDir == "" {
				continue
			}
			// 从 Meta 中获取二进制路径，或者尝试查找
			if binaryPath, ok := comp.Meta["binary"]; ok && binaryPath != "" {
				return binaryPath, nil
			}
			// 尝试自动查找
			return findCoreBinary(comp.InstallDir, kind)
		}
	}
	return "", fmt.Errorf("engine %s not installed", kind)
}

// findCoreBinary 在目录中查找内核二进制文件
func findCoreBinary(dir string, kind domain.CoreEngineKind) (string, error) {
	var candidates []string
	switch kind {
	case domain.EngineXray:
		candidates = []string{"xray", "xray.exe"}
	case domain.EngineSingBox:
		candidates = []string{"sing-box", "sing-box.exe"}
	}

	return findBinaryInDir(dir, candidates)
}

// findBinaryInDir 在目录中查找二进制文件（支持子目录）
func findBinaryInDir(dir string, candidates []string) (string, error) {
	// 先在根目录查找
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// 在子目录中查找（深度1层）
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("binary not found in %s (candidates: %v)", dir, candidates)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subdir := filepath.Join(dir, entry.Name())
		for _, name := range candidates {
			path := filepath.Join(subdir, name)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("binary not found in %s (candidates: %v)", dir, candidates)
}
