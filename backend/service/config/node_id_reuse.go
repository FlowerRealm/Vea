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

type existingNodeIDIndex struct {
	byFingerprint      map[string]string
	byIdentity         map[string]string
	byIdentityName     map[string]string
	ambiguousIdentites map[string]struct{}
	ambiguousNames     map[string]struct{}
}

func buildExistingNodeIDIndex(nodes []domain.Node) existingNodeIDIndex {
	if len(nodes) == 0 {
		return existingNodeIDIndex{}
	}

	byFingerprint := make(map[string]string, len(nodes))
	byIdentity := make(map[string]string, len(nodes))
	byIdentityName := make(map[string]string, len(nodes))
	ambiguousIdentites := make(map[string]struct{}, 8)
	ambiguousNames := make(map[string]struct{}, 8)

	for _, n := range nodes {
		id := strings.TrimSpace(n.ID)
		if id == "" {
			continue
		}

		key := fingerprintKey(n)
		if key != "" {
			if _, ok := byFingerprint[key]; !ok {
				byFingerprint[key] = id
			}
		}

		identity := identityKey(n)
		if identity == "" {
			continue
		}

		nameKey := identityNameKey(identity, n.Name)
		if nameKey != "" {
			if _, ok := ambiguousNames[nameKey]; ok {
				continue
			}
			if prev, ok := byIdentityName[nameKey]; ok && prev != id {
				ambiguousNames[nameKey] = struct{}{}
				delete(byIdentityName, nameKey)
			} else if !ok {
				byIdentityName[nameKey] = id
			}
		}

		if _, ok := ambiguousIdentites[identity]; ok {
			continue
		}
		if prev, ok := byIdentity[identity]; ok && prev != id {
			ambiguousIdentites[identity] = struct{}{}
			delete(byIdentity, identity)
			continue
		}
		if _, ok := byIdentity[identity]; !ok {
			byIdentity[identity] = id
		}
	}

	return existingNodeIDIndex{
		byFingerprint:      byFingerprint,
		byIdentity:         byIdentity,
		byIdentityName:     byIdentityName,
		ambiguousIdentites: ambiguousIdentites,
		ambiguousNames:     ambiguousNames,
	}
}

func reuseNodeIDs(index existingNodeIDIndex, nodes []domain.Node) ([]domain.Node, map[string]string) {
	if len(nodes) == 0 || (len(index.byFingerprint) == 0 && len(index.byIdentity) == 0 && len(index.byIdentityName) == 0) {
		return nodes, nil
	}

	used := make(map[string]struct{}, len(nodes))
	mapping := make(map[string]string, 8) // parsedID -> existingID

	for i := range nodes {
		originalID := strings.TrimSpace(nodes[i].ID)
		nextID := originalID

		key := fingerprintKey(nodes[i])
		if key != "" {
			if existingID := strings.TrimSpace(index.byFingerprint[key]); existingID != "" && existingID != originalID {
				if _, ok := used[existingID]; !ok {
					nextID = existingID
					if originalID != "" {
						mapping[originalID] = existingID
					}
				}
			}
		}

		if nextID == originalID {
			identity := identityKey(nodes[i])
			if identity != "" {
				if _, ok := index.ambiguousIdentites[identity]; ok {
					nameKey := identityNameKey(identity, nodes[i].Name)
					if nameKey != "" {
						if _, ok := index.ambiguousNames[nameKey]; !ok {
							if existingID := strings.TrimSpace(index.byIdentityName[nameKey]); existingID != "" && existingID != originalID {
								if _, ok := used[existingID]; !ok {
									nextID = existingID
									if originalID != "" {
										mapping[originalID] = existingID
									}
								}
							}
						}
					}
				} else if existingID := strings.TrimSpace(index.byIdentity[identity]); existingID != "" && existingID != originalID {
					if _, ok := used[existingID]; !ok {
						nextID = existingID
						if originalID != "" {
							mapping[originalID] = existingID
						}
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

func identityNameKey(identity string, name string) string {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return ""
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	return identity + "|" + name
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

func identityKey(node domain.Node) string {
	proto := node.Protocol
	addr := strings.ToLower(strings.TrimSpace(node.Address))
	if strings.TrimSpace(string(proto)) == "" || addr == "" || node.Port <= 0 {
		return ""
	}

	sec := canonicalSecurity(proto, node.Security)

	out := struct {
		Protocol  domain.NodeProtocol `json:"protocol"`
		Address   string              `json:"address"`
		Port      int                 `json:"port"`
		Transport string              `json:"transport,omitempty"`
		TLS       string              `json:"tls,omitempty"`
		UUID      string              `json:"uuid,omitempty"`
		Password  string              `json:"password,omitempty"`
		Method    string              `json:"method,omitempty"`
		Flow      string              `json:"flow,omitempty"`
	}{
		Protocol: proto,
		Address:  addr,
		Port:     node.Port,
	}

	if node.Transport != nil {
		typ := strings.ToLower(strings.TrimSpace(node.Transport.Type))
		if typ != "" && typ != "tcp" {
			out.Transport = typ
		}
	}
	if node.TLS != nil && node.TLS.Enabled {
		typ := strings.ToLower(strings.TrimSpace(node.TLS.Type))
		if typ == "" {
			typ = "tls"
		}
		out.TLS = typ
	}

	switch proto {
	case domain.ProtocolVLESS:
		if sec == nil || strings.TrimSpace(sec.UUID) == "" {
			return ""
		}
		out.UUID = strings.TrimSpace(sec.UUID)
		out.Flow = strings.TrimSpace(sec.Flow)
	case domain.ProtocolVMess:
		if sec == nil || strings.TrimSpace(sec.UUID) == "" {
			return ""
		}
		out.UUID = strings.TrimSpace(sec.UUID)
	case domain.ProtocolTrojan, domain.ProtocolHysteria2:
		if sec == nil || strings.TrimSpace(sec.Password) == "" {
			return ""
		}
		out.Password = strings.TrimSpace(sec.Password)
	case domain.ProtocolShadowsocks:
		if sec == nil || strings.TrimSpace(sec.Password) == "" || strings.TrimSpace(sec.Method) == "" {
			return ""
		}
		out.Password = strings.TrimSpace(sec.Password)
		out.Method = strings.TrimSpace(sec.Method)
	case domain.ProtocolTUIC:
		if sec == nil || strings.TrimSpace(sec.UUID) == "" || strings.TrimSpace(sec.Password) == "" {
			return ""
		}
		out.UUID = strings.TrimSpace(sec.UUID)
		out.Password = strings.TrimSpace(sec.Password)
	default:
		return ""
	}

	b, _ := json.Marshal(out)
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
