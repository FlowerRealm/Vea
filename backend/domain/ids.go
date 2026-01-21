package domain

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

// StableNodeIDForConfig 基于配置 ID 与节点指纹生成稳定的节点 ID。
// 注意：该函数用于“订阅/配置同步”场景；指纹内容与历史版本保持一致，避免无必要的 ID 漂移。
func StableNodeIDForConfig(configID string, node Node) string {
	type fingerprint struct {
		Protocol  NodeProtocol   `json:"protocol"`
		Address   string         `json:"address"`
		Port      int            `json:"port"`
		Security  *NodeSecurity  `json:"security,omitempty"`
		Transport *NodeTransport `json:"transport,omitempty"`
		TLS       *NodeTLS       `json:"tls,omitempty"`
	}
	b, _ := json.Marshal(fingerprint{
		Protocol:  node.Protocol,
		Address:   node.Address,
		Port:      node.Port,
		Security:  node.Security,
		Transport: node.Transport,
		TLS:       node.TLS,
	})
	return uuid.NewSHA1(uuid.NameSpaceOID, append([]byte(configID+"|"), b...)).String()
}

// StableNodeIDForSourceKey 基于配置 ID 与订阅 sourceKey 生成稳定的节点 ID。
//
// 适用场景：
// - 分享链接订阅：节点参数可能发生滚动更新（uuid/password/path/fp等），但用户希望按“订阅语义”保持稳定引用。
// - sourceKey 由订阅解析层生成并持久化到 Node 上，用于跨拉取稳定复用。
func StableNodeIDForSourceKey(configID string, sourceKey string) string {
	configID = strings.TrimSpace(configID)
	sourceKey = strings.ToLower(strings.TrimSpace(sourceKey))
	if configID == "" || sourceKey == "" {
		return ""
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(configID+"|sourceKey|"+sourceKey)).String()
}
