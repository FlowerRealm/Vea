package proxy

import (
	"path/filepath"

	"vea/backend/domain"
	"vea/backend/service/shared"
)

func engineArtifactsDirName(engine domain.CoreEngineKind) string {
	switch engine {
	case domain.EngineSingBox:
		// 组件安装目录与 rule-set 目录使用 "sing-box"（带连字符）。
		// EngineKind 仍保持 "singbox" 作为对外/状态字段，不做破坏性改名。
		return "sing-box"
	default:
		return string(engine)
	}
}

func engineConfigDir(engine domain.CoreEngineKind) string {
	return filepath.Join(shared.ArtifactsRoot, "core", engineArtifactsDirName(engine))
}
