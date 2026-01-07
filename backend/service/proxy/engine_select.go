package proxy

import (
	"context"
	"fmt"
	"strings"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/service/adapters"
	"vea/backend/service/nodegroup"
)

func selectEngineForFRouter(
	ctx context.Context,
	inboundMode domain.InboundMode,
	frouter domain.FRouter,
	nodes []domain.Node,
	preferred domain.CoreEngineKind,
	components repository.ComponentRepository,
	settings repository.SettingsRepository,
	adapters map[domain.CoreEngineKind]adapters.CoreAdapter,
) (domain.CoreEngineKind, domain.CoreComponent, error) {
	compiled, err := nodegroup.CompileFRouter(frouter, nodes)
	if err != nil {
		return "", domain.CoreComponent{}, fmt.Errorf("compile frouter: %w", err)
	}
	activeNodes := nodegroup.FilterNodesByID(nodes, nodegroup.ActiveNodeIDs(compiled))

	componentsList, err := components.List(ctx)
	if err != nil {
		return "", domain.CoreComponent{}, fmt.Errorf("list components: %w", err)
	}

	installed := installedEnginesFromComponents(componentsList)
	candidates := make([]domain.CoreEngineKind, 0, 4)
	addCandidate := func(engine domain.CoreEngineKind) {
		if engine == "" || engine == domain.EngineAuto {
			return
		}
		for _, existing := range candidates {
			if existing == engine {
				return
			}
		}
		candidates = append(candidates, engine)
	}

	if preferred != "" && preferred != domain.EngineAuto {
		adapter := adapters[preferred]
		if adapter == nil {
			return "", domain.CoreComponent{}, fmt.Errorf("内核适配器不存在: %s", preferred)
		}
		if inboundMode != "" && !adapter.SupportsInbound(inboundMode) {
			return "", domain.CoreComponent{}, fmt.Errorf("指定内核 %s 不支持入站模式 %s", preferred, inboundMode)
		}
		for _, node := range activeNodes {
			if !supportsNode(adapter, node) {
				name := strings.TrimSpace(node.Name)
				if name == "" {
					name = strings.TrimSpace(node.ID)
				}
				if name == "" {
					name = "unknown"
				}
				return "", domain.CoreComponent{}, fmt.Errorf("指定内核 %s 不支持节点 %s（%s）", preferred, name, node.Protocol)
			}
		}
		if comp, ok := installed[preferred]; ok {
			return preferred, comp, nil
		}
		// 允许“指定引擎但未安装”：由上层触发安装后重试启动。
		return preferred, domain.CoreComponent{}, nil
	}

	if len(candidates) == 0 && settings != nil {
		if settingsMap, err := settings.GetFrontend(ctx); err == nil {
			if defaultEngine, ok := settingsMap["engine.defaultEngine"].(string); ok {
				engine := domain.CoreEngineKind(defaultEngine)
				addCandidate(engine)
			}
		}
	}

	if len(candidates) == 0 {
		rec := recommendEngineForNodes(activeNodes, adapters)
		addCandidate(rec.RecommendedEngine)
	}

	addCandidate(domain.EngineSingBox)
	addCandidate(domain.EngineXray)
	addCandidate(domain.EngineClash)

	fallback := domain.CoreEngineKind("")
	for _, engine := range candidates {
		adapter := adapters[engine]
		if adapter == nil {
			continue
		}
		if !supportsAllNodes(adapter, inboundMode, activeNodes) {
			continue
		}

		comp, ok := installed[engine]
		if ok {
			return engine, comp, nil
		}
		if fallback == "" {
			fallback = engine
		}
	}

	if fallback != "" {
		return fallback, domain.CoreComponent{}, nil
	}
	return "", domain.CoreComponent{}, fmt.Errorf("no engine supports frouter nodes")
}

func supportsAllNodes(adapter adapters.CoreAdapter, inboundMode domain.InboundMode, nodes []domain.Node) bool {
	if adapter == nil {
		return false
	}
	if inboundMode != "" && !adapter.SupportsInbound(inboundMode) {
		return false
	}
	for _, node := range nodes {
		if !supportsNode(adapter, node) {
			return false
		}
	}
	return true
}

func supportsNode(adapter adapters.CoreAdapter, node domain.Node) bool {
	if adapter == nil {
		return false
	}
	if !adapter.SupportsProtocol(node.Protocol) {
		return false
	}

	// Shadowsocks 插件（如 obfs-local）：
	// - Xray 配置模型里不支持插件（无法表达/无法工作）
	// - sing-box / mihomo(clash) 支持
	if node.Protocol == domain.ProtocolShadowsocks && node.Security != nil && strings.TrimSpace(node.Security.Plugin) != "" {
		switch adapter.Kind() {
		case domain.EngineSingBox, domain.EngineClash:
			return true
		default:
			return false
		}
	}
	return true
}

func recommendEngineForNodes(nodes []domain.Node, adapters map[domain.CoreEngineKind]adapters.CoreAdapter) EngineRecommendation {
	// 无节点时默认推荐 Xray
	if len(nodes) == 0 {
		return EngineRecommendation{
			RecommendedEngine: domain.EngineXray,
			Reason:            "无节点，推荐使用更成熟稳定的 Xray",
			TotalNodes:        0,
		}
	}

	xrayAdapter := adapters[domain.EngineXray]
	var xrayCompatible, singBoxOnly int

	for _, node := range nodes {
		if supportsNode(xrayAdapter, node) {
			xrayCompatible++
		} else {
			singBoxOnly++
		}
	}

	rec := EngineRecommendation{
		XrayCompatible: xrayCompatible,
		SingBoxOnly:    singBoxOnly,
		TotalNodes:     len(nodes),
	}

	if singBoxOnly > 0 {
		rec.RecommendedEngine = domain.EngineSingBox
		rec.Reason = fmt.Sprintf("存在 %d 个 Xray 不支持的节点（如 Hysteria2/TUIC）", singBoxOnly)
	} else if xrayCompatible == len(nodes) {
		rec.RecommendedEngine = domain.EngineXray
		rec.Reason = "所有节点均支持 Xray，推荐使用更成熟稳定的 Xray"
	} else {
		rec.RecommendedEngine = domain.EngineSingBox
		rec.Reason = "sing-box 支持更多协议，推荐作为通用选择"
	}

	return rec
}
