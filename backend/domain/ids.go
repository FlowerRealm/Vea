package domain

import (
	"encoding/json"

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
