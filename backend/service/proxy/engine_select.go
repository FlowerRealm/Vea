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

	// Shadowsocks 插件（如 obfs-local）：sing-box 与 mihomo(clash) 支持。
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
	// 无节点时默认推荐 sing-box（项目默认内核）。
	if len(nodes) == 0 {
		return EngineRecommendation{
			RecommendedEngine: domain.EngineSingBox,
			Reason:            "无节点，默认使用 sing-box",
			TotalNodes:        0,
		}
	}

	singBoxAdapter := adapters[domain.EngineSingBox]
	clashAdapter := adapters[domain.EngineClash]

	var singBoxSupported, clashSupported int
	for _, node := range nodes {
		if supportsNode(singBoxAdapter, node) {
			singBoxSupported++
		}
		if supportsNode(clashAdapter, node) {
			clashSupported++
		}
	}

	total := len(nodes)

	// 规则：优先 sing-box（协议覆盖更广），不行则回退 clash(mihomo)。
	if singBoxAdapter != nil && singBoxSupported == total {
		return EngineRecommendation{
			RecommendedEngine: domain.EngineSingBox,
			Reason:            "sing-box 支持协议覆盖更广，推荐作为通用选择",
			TotalNodes:        total,
		}
	}

	if clashAdapter != nil && clashSupported == total {
		return EngineRecommendation{
			RecommendedEngine: domain.EngineClash,
			Reason:            "节点均可由 clash(mihomo) 支持，推荐使用 clash",
			TotalNodes:        total,
		}
	}

	// 部分兼容时：仍优先 sing-box（若适配器存在），否则回退 clash。
	if singBoxAdapter != nil {
		return EngineRecommendation{
			RecommendedEngine: domain.EngineSingBox,
			Reason:            fmt.Sprintf("sing-box 可支持 %d/%d 个节点，优先作为默认选择", singBoxSupported, total),
			TotalNodes:        total,
		}
	}
	if clashAdapter != nil {
		return EngineRecommendation{
			RecommendedEngine: domain.EngineClash,
			Reason:            fmt.Sprintf("clash 可支持 %d/%d 个节点，作为兜底选择", clashSupported, total),
			TotalNodes:        total,
		}
	}

	return EngineRecommendation{
		RecommendedEngine: "",
		Reason:            "未找到可用内核适配器",
		TotalNodes:        total,
	}
}
