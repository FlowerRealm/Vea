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
		addCandidate(preferred)
	}

	if anyNodeRequiresSingBox(activeNodes) {
		addCandidate(domain.EngineSingBox)
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

	for _, engine := range candidates {
		comp, ok := installed[engine]
		if !ok {
			continue
		}
		adapter := adapters[engine]
		if adapter == nil {
			continue
		}
		if !supportsAllNodes(adapter, activeNodes) {
			continue
		}
		return engine, comp, nil
	}

	return "", domain.CoreComponent{}, fmt.Errorf("no installed engine supports frouter nodes")
}

func anyNodeRequiresSingBox(nodes []domain.Node) bool {
	for _, node := range nodes {
		if requiresSingBoxForNode(node) {
			return true
		}
	}
	return false
}

func requiresSingBoxForNode(node domain.Node) bool {
	switch node.Protocol {
	case domain.ProtocolHysteria2, domain.ProtocolTUIC:
		return true
	case domain.ProtocolShadowsocks:
		if node.Security == nil {
			return false
		}
		return strings.TrimSpace(node.Security.Plugin) != ""
	default:
		return false
	}
}

func supportsAllNodes(adapter adapters.CoreAdapter, nodes []domain.Node) bool {
	if adapter == nil {
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
	if requiresSingBoxForNode(node) && adapter.Kind() != domain.EngineSingBox {
		return false
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
		rec.Reason = fmt.Sprintf("存在 %d 个仅 sing-box 支持的节点（如 Hysteria2/TUIC）", singBoxOnly)
	} else if xrayCompatible == len(nodes) {
		rec.RecommendedEngine = domain.EngineXray
		rec.Reason = "所有节点均支持 Xray，推荐使用更成熟稳定的 Xray"
	} else {
		rec.RecommendedEngine = domain.EngineSingBox
		rec.Reason = "sing-box 支持更多协议，推荐作为通用选择"
	}

	return rec
}
