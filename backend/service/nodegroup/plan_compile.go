package nodegroup

import (
	"time"

	"vea/backend/domain"
)

func CompileProxyPlan(engine domain.CoreEngineKind, cfg domain.ProxyConfig, frouter domain.FRouter, nodes []domain.Node) (RuntimePlan, error) {
	compiled, err := CompileFRouter(frouter, nodes)
	if err != nil {
		return RuntimePlan{}, err
	}
	planNodes := FilterNodesByID(nodes, ActiveNodeIDs(compiled))
	return RuntimePlan{
		Purpose:     PurposeProxy,
		Engine:      engine,
		ProxyConfig: cfg,
		FRouterID:   frouter.ID,
		FRouterName: frouter.Name,
		Nodes:       planNodes,
		Compiled:    compiled,
		InboundMode: cfg.InboundMode,
		InboundPort: cfg.InboundPort,
		CreatedAt:   time.Now(),
	}, nil
}

func CompileMeasurementPlan(engine domain.CoreEngineKind, inboundPort int, frouter domain.FRouter, nodes []domain.Node) (RuntimePlan, error) {
	compiled, err := CompileFRouter(frouter, nodes)
	if err != nil {
		return RuntimePlan{}, err
	}
	planNodes := FilterNodesByID(nodes, ActiveNodeIDs(compiled))
	return RuntimePlan{
		Purpose:     PurposeMeasurement,
		Engine:      engine,
		FRouterID:   frouter.ID,
		FRouterName: frouter.Name,
		Nodes:       planNodes,
		Compiled:    compiled,
		InboundMode: domain.InboundSOCKS,
		InboundPort: inboundPort,
		CreatedAt:   time.Now(),
	}, nil
}
