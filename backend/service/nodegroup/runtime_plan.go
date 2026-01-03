package nodegroup

import (
	"strconv"
	"strings"
	"time"

	"vea/backend/domain"
)

// Purpose 表示本次计划的用途（主代理 / 测速等）。
type Purpose string

const (
	PurposeProxy       Purpose = "proxy"
	PurposeMeasurement Purpose = "measurement"
)

// RuntimePlan 是“节点群黑盒”的编译产物（引擎无关的中间表示）。
// 目前先作为最小可落地的骨架，后续逐步把业务决策从 adapters 中迁移到这里。
type RuntimePlan struct {
	Purpose     Purpose
	Engine      domain.CoreEngineKind
	ProxyConfig domain.ProxyConfig

	FRouterID   string
	FRouterName string
	Nodes       []domain.Node
	Compiled    CompiledFRouter

	InboundMode domain.InboundMode
	InboundPort int

	CreatedAt time.Time
}

// Explain 返回用于排障/提示的可读摘要（不会包含敏感信息）。
func (p RuntimePlan) Explain() string {
	var b strings.Builder
	b.WriteString("purpose=")
	b.WriteString(string(p.Purpose))
	b.WriteString("\nengine=")
	b.WriteString(string(p.Engine))
	if p.FRouterID != "" {
		b.WriteString("\nfrouter=")
		b.WriteString(p.FRouterID)
	}
	if p.InboundMode != "" {
		b.WriteString("\ninboundMode=")
		b.WriteString(string(p.InboundMode))
	}
	if p.InboundPort > 0 {
		b.WriteString("\ninboundPort=")
		b.WriteString(strconv.Itoa(p.InboundPort))
	}
	if len(p.Nodes) > 0 {
		b.WriteString("\nnodes=")
		b.WriteString(strconv.Itoa(len(p.Nodes)))
	}
	b.WriteString("\nrouteRules=")
	b.WriteString(strconv.Itoa(len(p.Compiled.Rules)))
	b.WriteString("\ndetours=")
	b.WriteString(strconv.Itoa(len(p.Compiled.DetourUpstream)))
	if len(p.Compiled.Warnings) > 0 {
		b.WriteString("\nwarnings=")
		b.WriteString(strconv.Itoa(len(p.Compiled.Warnings)))
	}
	if p.Compiled.Default.Kind != "" {
		b.WriteString("\ndefault=")
		b.WriteString(p.Compiled.Default.String())
	}
	return b.String()
}
