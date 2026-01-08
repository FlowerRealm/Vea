package persist

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vea/backend/domain"
)

// SchemaVersion 当前架构版本
const SchemaVersion = "2.1.0"

const legacySchemaVersion_2_0_0 = "2.0.0"

type legacyFRouter_2_0_0 struct {
	ID               string                    `json:"id"`
	Name             string                    `json:"name"`
	Nodes            []domain.Node             `json:"nodes"`
	ChainProxy       domain.ChainProxySettings `json:"chainProxy"`
	Tags             []string                  `json:"tags,omitempty"`
	SourceConfigID   string                    `json:"sourceConfigId,omitempty"`
	LastLatencyMS    int64                     `json:"lastLatencyMs"`
	LastLatencyAt    time.Time                 `json:"lastLatencyAt"`
	LastLatencyError string                    `json:"lastLatencyError,omitempty"`
	LastSpeedMbps    float64                   `json:"lastSpeedMbps"`
	LastSpeedAt      time.Time                 `json:"lastSpeedAt"`
	LastSpeedError   string                    `json:"lastSpeedError,omitempty"`
	CreatedAt        time.Time                 `json:"createdAt"`
	UpdatedAt        time.Time                 `json:"updatedAt"`
}

type legacyServiceState_2_0_0 struct {
	SchemaVersion    string                     `json:"schemaVersion,omitempty"`
	Nodes            []domain.Node              `json:"nodes"`
	FRouters         []legacyFRouter_2_0_0      `json:"frouters"`
	Configs          []domain.Config            `json:"configs"`
	GeoResources     []domain.GeoResource       `json:"geoResources"`
	Components       []domain.CoreComponent     `json:"components"`
	SystemProxy      domain.SystemProxySettings `json:"systemProxy"`
	ProxyConfig      domain.ProxyConfig         `json:"proxyConfig"`
	FrontendSettings map[string]interface{}     `json:"frontendSettings,omitempty"`
	GeneratedAt      time.Time                  `json:"generatedAt"`
}

// Migrator 版本校验器（仅接受当前 schemaVersion）
type Migrator struct{}

// NewMigrator 创建校验器
func NewMigrator() *Migrator {
	return &Migrator{}
}

// Migrate 解析并校验版本
func (m *Migrator) Migrate(data []byte) (domain.ServiceState, error) {
	if len(data) == 0 {
		return domain.ServiceState{SchemaVersion: SchemaVersion}, nil
	}

	// 先只解析 schemaVersion，避免直接丢字段导致不可逆数据丢失。
	var meta struct {
		SchemaVersion string `json:"schemaVersion,omitempty"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return domain.ServiceState{}, fmt.Errorf("failed to parse state: %w", err)
	}
	if meta.SchemaVersion == "" {
		// 兼容历史 state.json：早期版本未写入 schemaVersion。
		// 这里按当前结构尽力解析（未知字段会被忽略），避免启动即“读不了 -> 覆盖空 state”的灾难。
		var state domain.ServiceState
		if err := json.Unmarshal(data, &state); err != nil {
			return domain.ServiceState{}, fmt.Errorf("failed to parse legacy state: %w", err)
		}
		state.SchemaVersion = SchemaVersion
		if state.GeneratedAt.IsZero() {
			state.GeneratedAt = time.Now()
		}
		return sanitizeServiceState(state), nil
	}

	switch meta.SchemaVersion {
	case SchemaVersion:
		var state domain.ServiceState
		if err := json.Unmarshal(data, &state); err != nil {
			return domain.ServiceState{}, fmt.Errorf("failed to parse state: %w", err)
		}
		return sanitizeServiceState(state), nil
	case legacySchemaVersion_2_0_0:
		var legacy legacyServiceState_2_0_0
		if err := json.Unmarshal(data, &legacy); err != nil {
			return domain.ServiceState{}, fmt.Errorf("failed to parse legacy state: %w", err)
		}
		return sanitizeServiceState(migrate_2_0_0_to_2_1_0(legacy)), nil
	default:
		return domain.ServiceState{}, fmt.Errorf("unsupported schemaVersion %s (expected %s)", meta.SchemaVersion, SchemaVersion)
	}
}

func migrate_2_0_0_to_2_1_0(legacy legacyServiceState_2_0_0) domain.ServiceState {
	nodesByID := make(map[string]domain.Node, 256)

	for _, n := range legacy.Nodes {
		if n.ID == "" {
			continue
		}
		if _, ok := nodesByID[n.ID]; ok {
			continue
		}
		nodesByID[n.ID] = n
	}

	nextFRouters := make([]domain.FRouter, 0, len(legacy.FRouters))
	for _, fr := range legacy.FRouters {
		for _, n := range fr.Nodes {
			if n.ID == "" {
				continue
			}
			if _, ok := nodesByID[n.ID]; ok {
				continue
			}
			// 继承 frouter 的 sourceConfigId（旧状态中 node 可能没写）
			if fr.SourceConfigID != "" && n.SourceConfigID == "" {
				n.SourceConfigID = fr.SourceConfigID
			}
			nodesByID[n.ID] = n
		}

		nextFRouters = append(nextFRouters, domain.FRouter{
			ID:               fr.ID,
			Name:             fr.Name,
			ChainProxy:       fr.ChainProxy,
			Tags:             fr.Tags,
			SourceConfigID:   fr.SourceConfigID,
			LastLatencyMS:    fr.LastLatencyMS,
			LastLatencyAt:    fr.LastLatencyAt,
			LastLatencyError: fr.LastLatencyError,
			LastSpeedMbps:    fr.LastSpeedMbps,
			LastSpeedAt:      fr.LastSpeedAt,
			LastSpeedError:   fr.LastSpeedError,
			CreatedAt:        fr.CreatedAt,
			UpdatedAt:        fr.UpdatedAt,
		})
	}

	nextNodes := make([]domain.Node, 0, len(nodesByID))
	for _, n := range nodesByID {
		nextNodes = append(nextNodes, n)
	}

	return domain.ServiceState{
		SchemaVersion:    SchemaVersion,
		Nodes:            nextNodes,
		FRouters:         nextFRouters,
		Configs:          legacy.Configs,
		GeoResources:     legacy.GeoResources,
		Components:       legacy.Components,
		SystemProxy:      legacy.SystemProxy,
		ProxyConfig:      legacy.ProxyConfig,
		FrontendSettings: legacy.FrontendSettings,
		GeneratedAt:      time.Now(),
	}
}

func sanitizeServiceState(state domain.ServiceState) domain.ServiceState {
	// 配置格式：旧版本可能写入 xray-json（历史遗留），统一归一化为 subscription。
	for i := range state.Configs {
		format := strings.TrimSpace(string(state.Configs[i].Format))
		if format == "" || format == "xray-json" {
			state.Configs[i].Format = domain.ConfigFormatSubscription
		} else if state.Configs[i].Format != domain.ConfigFormatSubscription {
			// 兜底：不保留未知格式，避免前端/SDK 枚举不一致。
			state.Configs[i].Format = domain.ConfigFormatSubscription
		}
	}

	// 引擎：移除 Xray 后，旧状态里的 preferredEngine=xray 必须回退。
	switch state.ProxyConfig.PreferredEngine {
	case "", domain.EngineAuto, domain.EngineSingBox, domain.EngineClash:
	default:
		state.ProxyConfig.PreferredEngine = domain.EngineAuto
	}

	// 前端设置：移除 Xray 后，清理 xray.* 设置与默认引擎 xray。
	if state.FrontendSettings != nil {
		if v, ok := state.FrontendSettings["engine.defaultEngine"].(string); ok {
			if strings.EqualFold(strings.TrimSpace(v), "xray") {
				// 选择最接近的默认引擎：sing-box。
				state.FrontendSettings["engine.defaultEngine"] = string(domain.EngineSingBox)
			}
		}
		for k := range state.FrontendSettings {
			if strings.HasPrefix(k, "xray.") {
				delete(state.FrontendSettings, k)
			}
		}
	}

	// 组件：过滤掉 xray 组件，避免前端仍展示“Xray”或后端误处理。
	if len(state.Components) > 0 {
		next := make([]domain.CoreComponent, 0, len(state.Components))
		for _, comp := range state.Components {
			if strings.EqualFold(strings.TrimSpace(string(comp.Kind)), "xray") {
				continue
			}
			next = append(next, comp)
		}
		state.Components = next
	}

	return state
}
