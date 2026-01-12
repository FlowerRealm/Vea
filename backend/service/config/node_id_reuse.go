package config

import (
	"encoding/json"
	"strings"

	"vea/backend/domain"
)

type nodeFingerprint struct {
	Protocol  domain.NodeProtocol   `json:"protocol"`
	Address   string                `json:"address"`
	Port      int                   `json:"port"`
	Security  *domain.NodeSecurity  `json:"security,omitempty"`
	Transport *domain.NodeTransport `json:"transport,omitempty"`
	TLS       *domain.NodeTLS       `json:"tls,omitempty"`
}

func buildExistingNodeIDByFingerprint(nodes []domain.Node) map[string]string {
	out := make(map[string]string, len(nodes))
	for _, n := range nodes {
		id := strings.TrimSpace(n.ID)
		if id == "" {
			continue
		}
		key := fingerprintKey(n)
		if key == "" {
			continue
		}
		if _, ok := out[key]; ok {
			continue
		}
		out[key] = id
	}
	return out
}

func reuseNodeIDs(existingIDByFingerprint map[string]string, nodes []domain.Node) ([]domain.Node, map[string]string) {
	if len(nodes) == 0 || len(existingIDByFingerprint) == 0 {
		return nodes, nil
	}

	used := make(map[string]struct{}, len(nodes))
	mapping := make(map[string]string, 8) // oldID -> newID

	for i := range nodes {
		originalID := strings.TrimSpace(nodes[i].ID)
		nextID := originalID

		key := fingerprintKey(nodes[i])
		if key != "" {
			if existingID := strings.TrimSpace(existingIDByFingerprint[key]); existingID != "" && existingID != originalID {
				if _, ok := used[existingID]; !ok {
					nextID = existingID
					if originalID != "" {
						mapping[originalID] = existingID
					}
				}
			}
		}

		nodes[i].ID = nextID
		if nextID != "" {
			used[nextID] = struct{}{}
		}
	}

	if len(mapping) == 0 {
		return nodes, nil
	}
	return nodes, mapping
}

func fingerprintKey(node domain.Node) string {
	fp := nodeFingerprint{
		Protocol:  node.Protocol,
		Address:   strings.ToLower(strings.TrimSpace(node.Address)),
		Port:      node.Port,
		Security:  canonicalSecurity(node.Protocol, node.Security),
		Transport: canonicalTransport(node.Transport),
		TLS:       canonicalTLS(node.TLS),
	}
	b, _ := json.Marshal(fp)
	return string(b)
}

func canonicalSecurity(proto domain.NodeProtocol, sec *domain.NodeSecurity) *domain.NodeSecurity {
	if sec == nil {
		return nil
	}

	out := &domain.NodeSecurity{
		UUID:       strings.TrimSpace(sec.UUID),
		Password:   strings.TrimSpace(sec.Password),
		Method:     strings.TrimSpace(sec.Method),
		Flow:       strings.TrimSpace(sec.Flow),
		Encryption: strings.TrimSpace(sec.Encryption),
		AlterID:    sec.AlterID,
		Plugin:     strings.TrimSpace(sec.Plugin),
		PluginOpts: strings.TrimSpace(sec.PluginOpts),
	}

	if len(sec.ALPN) > 0 {
		alpn := make([]string, 0, len(sec.ALPN))
		for _, v := range sec.ALPN {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			alpn = append(alpn, v)
		}
		if len(alpn) > 0 {
			out.ALPN = alpn
		}
	}

	switch proto {
	case domain.ProtocolVLESS:
		if out.Encryption == "" {
			out.Encryption = "none"
		}
		out.Method = ""
	case domain.ProtocolVMess:
		cipher := out.Encryption
		if cipher == "" {
			cipher = out.Method
		}
		if cipher == "" {
			cipher = "auto"
		}
		out.Method = cipher
		out.Encryption = cipher
	case domain.ProtocolShadowsocks:
		method := out.Method
		if method == "" {
			method = out.Encryption
		}
		out.Method = method
		out.Encryption = method
	}

	if out.UUID == "" &&
		out.Password == "" &&
		out.Method == "" &&
		out.Flow == "" &&
		out.Encryption == "" &&
		out.AlterID == 0 &&
		out.Plugin == "" &&
		out.PluginOpts == "" &&
		len(out.ALPN) == 0 {
		return nil
	}
	return out
}

func canonicalTransport(t *domain.NodeTransport) *domain.NodeTransport {
	if t == nil {
		return nil
	}

	out := &domain.NodeTransport{
		Type:        strings.ToLower(strings.TrimSpace(t.Type)),
		Host:        strings.TrimSpace(t.Host),
		Path:        strings.TrimSpace(t.Path),
		ServiceName: strings.TrimSpace(t.ServiceName),
		HeaderType:  strings.ToLower(strings.TrimSpace(t.HeaderType)),
	}

	if len(t.Headers) > 0 {
		headers := make(map[string]string, len(t.Headers))
		for k, v := range t.Headers {
			key := strings.TrimSpace(k)
			if key == "" {
				continue
			}
			val := strings.TrimSpace(v)
			if val == "" {
				continue
			}
			headers[key] = val
		}
		if len(headers) > 0 {
			out.Headers = headers
		}
	}

	if out.Type == "" || out.Type == "tcp" {
		if out.Host == "" && out.Path == "" && out.ServiceName == "" && out.HeaderType == "" && len(out.Headers) == 0 {
			return nil
		}
	}
	return out
}

func canonicalTLS(tls *domain.NodeTLS) *domain.NodeTLS {
	if tls == nil {
		return nil
	}

	out := &domain.NodeTLS{
		Enabled:          tls.Enabled,
		Type:             strings.ToLower(strings.TrimSpace(tls.Type)),
		ServerName:       strings.TrimSpace(tls.ServerName),
		Insecure:         tls.Insecure,
		Fingerprint:      strings.TrimSpace(tls.Fingerprint),
		RealityPublicKey: strings.TrimSpace(tls.RealityPublicKey),
		RealityShortID:   strings.TrimSpace(tls.RealityShortID),
	}

	if len(tls.ALPN) > 0 {
		alpn := make([]string, 0, len(tls.ALPN))
		for _, v := range tls.ALPN {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			alpn = append(alpn, v)
		}
		if len(alpn) > 0 {
			out.ALPN = alpn
		}
	}

	if !out.Enabled &&
		out.Type == "" &&
		out.ServerName == "" &&
		!out.Insecure &&
		out.Fingerprint == "" &&
		out.RealityPublicKey == "" &&
		out.RealityShortID == "" &&
		len(out.ALPN) == 0 {
		return nil
	}
	return out
}

func rewriteChainProxyNodeIDs(chain domain.ChainProxySettings, idMap map[string]string) domain.ChainProxySettings {
	if len(idMap) == 0 {
		return chain
	}

	rewriteRef := func(id string) string {
		id = strings.TrimSpace(id)
		if id == "" {
			return id
		}
		if id == domain.EdgeNodeLocal || id == domain.EdgeNodeDirect || id == domain.EdgeNodeBlock || domain.IsSlotNode(id) {
			return id
		}
		if next, ok := idMap[id]; ok && strings.TrimSpace(next) != "" {
			return next
		}
		return id
	}

	if len(chain.Edges) > 0 {
		edges := make([]domain.ProxyEdge, len(chain.Edges))
		copy(edges, chain.Edges)
		for i := range edges {
			edges[i].From = rewriteRef(edges[i].From)
			edges[i].To = rewriteRef(edges[i].To)
			if len(edges[i].Via) > 0 {
				via := make([]string, len(edges[i].Via))
				for j, hop := range edges[i].Via {
					via[j] = rewriteRef(hop)
				}
				edges[i].Via = via
			}
		}
		chain.Edges = edges
	}

	if len(chain.Slots) > 0 {
		slots := make([]domain.SlotNode, len(chain.Slots))
		copy(slots, chain.Slots)
		for i := range slots {
			slots[i].BoundNodeID = rewriteRef(slots[i].BoundNodeID)
		}
		chain.Slots = slots
	}

	if len(chain.Positions) > 0 {
		pos := make(map[string]domain.GraphPosition, len(chain.Positions))
		for k, v := range chain.Positions {
			pos[rewriteRef(k)] = v
		}
		chain.Positions = pos
	}

	return chain
}
