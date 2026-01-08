package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"vea/backend/domain"
	"vea/backend/repository/events"
	"vea/backend/repository/memory"
	"vea/backend/service/nodes"
)

func TestService_Create_WithSourceURL_DownloadsPayloadAndChecksum(t *testing.T) {
	t.Parallel()

	const payload = "hello"
	var gotUserAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.UserAgent()
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	repo := memory.NewConfigRepo(memory.NewStore(events.NewBus()))
	svc := NewService(repo, nil, nil)

	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if gotUserAgent != subscriptionUserAgent {
		t.Fatalf("expected User-Agent %q, got %q", subscriptionUserAgent, gotUserAgent)
	}
	if created.Payload != payload {
		t.Fatalf("expected payload %q, got %q", payload, created.Payload)
	}
	sum := sha256.Sum256([]byte(payload))
	expectedChecksum := hex.EncodeToString(sum[:])
	if created.Checksum != expectedChecksum {
		t.Fatalf("expected checksum %q, got %q", expectedChecksum, created.Checksum)
	}
	if created.LastSyncedAt.IsZero() {
		t.Fatalf("expected lastSyncedAt to be set")
	}
	if created.LastSyncError != "" {
		t.Fatalf("expected lastSyncError empty, got %q", created.LastSyncError)
	}
}

func TestService_Sync_UnchangedChecksum_OnlyUpdatesLastSyncedAt(t *testing.T) {
	t.Parallel()

	const payload = "same-content"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	repo := memory.NewConfigRepo(memory.NewStore(events.NewBus()))
	svc := NewService(repo, nil, nil)

	sum := sha256.Sum256([]byte(payload))
	checksum := hex.EncodeToString(sum[:])
	created, err := repo.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
		Payload:   payload,
		Checksum:  checksum,
		// 让 Sync “确实有更新”的可比较基准
		LastSyncedAt: time.Now().Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.Sync(context.Background(), created.ID); err != nil {
		t.Fatalf("sync: %v", err)
	}
	updated, err := repo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.Payload != payload {
		t.Fatalf("expected payload unchanged %q, got %q", payload, updated.Payload)
	}
	if updated.Checksum != created.Checksum {
		t.Fatalf("expected checksum unchanged %q, got %q", created.Checksum, updated.Checksum)
	}
	if !updated.LastSyncedAt.After(created.LastSyncedAt) {
		t.Fatalf("expected lastSyncedAt to move forward, before=%v after=%v", created.LastSyncedAt, updated.LastSyncedAt)
	}
	if updated.LastSyncError != "" {
		t.Fatalf("expected lastSyncError empty, got %q", updated.LastSyncError)
	}
}

func TestService_Sync_ParseFailure_DoesNotClearExistingNodes(t *testing.T) {
	t.Parallel()

	var payload atomic.Value
	payload.Store("")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, payload.Load().(string))
	}))
	t.Cleanup(srv.Close)

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	frouterRepo := memory.NewFRouterRepo(store)
	nodeSvc := nodes.NewService(nodeRepo)
	svc := NewService(configRepo, nodeSvc, frouterRepo)

	payload.Store("vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls#n1")
	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	nodesBefore, err := nodeRepo.ListByConfigID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list nodes before: %v", err)
	}
	if len(nodesBefore) != 1 {
		t.Fatalf("expected nodes=1 before sync, got %d", len(nodesBefore))
	}

	payload.Store("port: 7890\nsocks-port: 7891\nProxy:\n  - name: 您的客户端版本过旧\n    type: socks5\n    server: 127.0.0.1\n    port: 1080\n")
	if err := svc.Sync(context.Background(), created.ID); err == nil {
		t.Fatalf("expected sync to fail on unsupported subscription payload")
	}

	nodesAfter, err := nodeRepo.ListByConfigID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list nodes after: %v", err)
	}
	if len(nodesAfter) != 1 {
		t.Fatalf("expected nodes preserved after parse failure, got %d", len(nodesAfter))
	}

	updatedCfg, err := configRepo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if updatedCfg.LastSyncError == "" {
		t.Fatalf("expected lastSyncError to be set on parse failure")
	}
}

func TestService_Sync_EmptyPayload_DoesNotClearExistingNodes(t *testing.T) {
	t.Parallel()

	var payload atomic.Value
	payload.Store("")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, payload.Load().(string))
	}))
	t.Cleanup(srv.Close)

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	frouterRepo := memory.NewFRouterRepo(store)
	nodeSvc := nodes.NewService(nodeRepo)
	svc := NewService(configRepo, nodeSvc, frouterRepo)

	payload.Store("vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls#n1")
	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	nodesBefore, err := nodeRepo.ListByConfigID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list nodes before: %v", err)
	}
	if len(nodesBefore) != 1 {
		t.Fatalf("expected nodes=1 before sync, got %d", len(nodesBefore))
	}

	payload.Store("  \n\t")
	if err := svc.Sync(context.Background(), created.ID); err == nil {
		t.Fatalf("expected sync to fail on empty subscription payload")
	}

	nodesAfter, err := nodeRepo.ListByConfigID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list nodes after: %v", err)
	}
	if len(nodesAfter) != 1 {
		t.Fatalf("expected nodes preserved after empty payload, got %d", len(nodesAfter))
	}

	updatedCfg, err := configRepo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if updatedCfg.Payload != created.Payload {
		t.Fatalf("expected payload preserved on empty sync, got %q vs %q", updatedCfg.Payload, created.Payload)
	}
	if updatedCfg.Checksum != created.Checksum {
		t.Fatalf("expected checksum preserved on empty sync, got %q vs %q", updatedCfg.Checksum, created.Checksum)
	}
	if updatedCfg.LastSyncError == "" {
		t.Fatalf("expected lastSyncError to be set on empty payload")
	}
}

func TestService_Create_ClashYAML_GeneratesNodesAndFRouter(t *testing.T) {
	t.Parallel()

	const payload = `
proxies:
  - name: n1
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - n1
rules:
  - DOMAIN-SUFFIX,google.com,PROXY
  - MATCH,PROXY
`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, payload)
	}))
	t.Cleanup(srv.Close)

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	frouterRepo := memory.NewFRouterRepo(store)
	nodeSvc := nodes.NewService(nodeRepo)
	svc := NewService(configRepo, nodeSvc, frouterRepo)

	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	createdNodes, err := nodeRepo.ListByConfigID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(createdNodes) != 1 {
		t.Fatalf("expected nodes=1, got %d", len(createdNodes))
	}
	if createdNodes[0].Protocol != domain.ProtocolVMess {
		t.Fatalf("expected protocol=%s, got %s", domain.ProtocolVMess, createdNodes[0].Protocol)
	}

	frouterID := stableFRouterIDForConfig(created.ID)
	frouter, err := frouterRepo.Get(context.Background(), frouterID)
	if err != nil {
		t.Fatalf("get frouter: %v", err)
	}
	if frouter.SourceConfigID != created.ID {
		t.Fatalf("expected sourceConfigId=%q, got %q", created.ID, frouter.SourceConfigID)
	}
	if len(frouter.ChainProxy.Edges) != 2 {
		t.Fatalf("expected edges=2, got %d", len(frouter.ChainProxy.Edges))
	}

	slotID := stableSlotIDForConfig(created.ID, "PROXY")
	if len(frouter.ChainProxy.Slots) != 1 {
		t.Fatalf("expected slots=1, got %d", len(frouter.ChainProxy.Slots))
	}
	if frouter.ChainProxy.Slots[0].ID != slotID {
		t.Fatalf("expected slotId=%q, got %q", slotID, frouter.ChainProxy.Slots[0].ID)
	}
	if frouter.ChainProxy.Slots[0].BoundNodeID != createdNodes[0].ID {
		t.Fatalf("expected slot boundNodeId=%q, got %q", createdNodes[0].ID, frouter.ChainProxy.Slots[0].BoundNodeID)
	}
}

func TestService_Create_ClashYAML_CompactsConsecutiveRulesByTarget(t *testing.T) {
	t.Parallel()

	const payload = `
proxies:
  - name: n1
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - n1
rules:
  - DOMAIN-SUFFIX,google.com,PROXY
  - DOMAIN-SUFFIX,youtube.com,PROXY
  - DOMAIN-SUFFIX,baidu.com,DIRECT
  - DOMAIN-SUFFIX,example.com,PROXY
  - MATCH,PROXY
`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, payload)
	}))
	t.Cleanup(srv.Close)

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	frouterRepo := memory.NewFRouterRepo(store)
	nodeSvc := nodes.NewService(nodeRepo)
	svc := NewService(configRepo, nodeSvc, frouterRepo)

	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-compact",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	createdNodes, err := nodeRepo.ListByConfigID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(createdNodes) != 1 {
		t.Fatalf("expected nodes=1, got %d", len(createdNodes))
	}

	frouterID := stableFRouterIDForConfig(created.ID)
	frouter, err := frouterRepo.Get(context.Background(), frouterID)
	if err != nil {
		t.Fatalf("get frouter: %v", err)
	}
	// 4 条路由规则：前两条同去向合并为 1 条；默认边 1 条 → 合计 4 条
	if len(frouter.ChainProxy.Edges) != 4 {
		t.Fatalf("expected edges=4 after compaction, got %d", len(frouter.ChainProxy.Edges))
	}

	slotID := stableSlotIDForConfig(created.ID, "PROXY")
	proxyEdges := make([]domain.ProxyEdge, 0, 2)
	for _, edge := range frouter.ChainProxy.Edges {
		if edge.RuleType == domain.EdgeRuleRoute && edge.To == slotID {
			proxyEdges = append(proxyEdges, edge)
		}
	}
	if len(proxyEdges) != 2 {
		t.Fatalf("expected proxy edges=2 after compaction, got %d", len(proxyEdges))
	}

	contains := func(list []string, want string) bool {
		for _, item := range list {
			if item == want {
				return true
			}
		}
		return false
	}

	var mergedProxy domain.ProxyEdge
	foundMerged := false
	foundSolo := false
	for _, edge := range proxyEdges {
		if edge.RouteRule == nil {
			continue
		}
		if contains(edge.RouteRule.Domains, "domain:google.com") {
			mergedProxy = edge
			foundMerged = true
		}
		if contains(edge.RouteRule.Domains, "domain:example.com") {
			foundSolo = true
			if len(edge.RouteRule.Domains) != 1 {
				t.Fatalf("expected solo proxy edge to have 1 domain, got %d", len(edge.RouteRule.Domains))
			}
		}
	}
	if !foundMerged {
		t.Fatalf("expected merged proxy edge to include domain:google.com")
	}
	if mergedProxy.RouteRule == nil || !contains(mergedProxy.RouteRule.Domains, "domain:youtube.com") {
		t.Fatalf("expected merged proxy edge to include domain:youtube.com")
	}
	if len(mergedProxy.RouteRule.Domains) != 2 {
		t.Fatalf("expected merged proxy edge domains=2, got %d", len(mergedProxy.RouteRule.Domains))
	}
	if !foundSolo {
		t.Fatalf("expected separate proxy edge to include domain:example.com")
	}

	foundDirect := false
	for _, edge := range frouter.ChainProxy.Edges {
		if edge.RuleType != domain.EdgeRuleRoute || edge.To != domain.EdgeNodeDirect || edge.RouteRule == nil {
			continue
		}
		foundDirect = true
		if !contains(edge.RouteRule.Domains, "domain:baidu.com") {
			t.Fatalf("expected direct edge to include domain:baidu.com")
		}
	}
	if !foundDirect {
		t.Fatalf("expected a direct edge to exist")
	}
}

func TestService_Create_ClashYAML_ShadowsocksObfsNormalizesToObfsLocal(t *testing.T) {
	t.Parallel()

	const payload = `
proxies:
  - name: ss-obfs
    type: ss
    server: example.com
    port: 443
    cipher: aes-128-gcm
    password: pass
    plugin: obfs
    plugin-opts:
      mode: tls
      host: obfs.example.com
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - ss-obfs
rules:
  - MATCH,PROXY
`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, payload)
	}))
	t.Cleanup(srv.Close)

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	frouterRepo := memory.NewFRouterRepo(store)
	nodeSvc := nodes.NewService(nodeRepo)
	svc := NewService(configRepo, nodeSvc, frouterRepo)

	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-ss-obfs",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	createdNodes, err := nodeRepo.ListByConfigID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(createdNodes) != 1 {
		t.Fatalf("expected nodes=1, got %d", len(createdNodes))
	}
	n := createdNodes[0]
	if n.Protocol != domain.ProtocolShadowsocks {
		t.Fatalf("expected protocol=%s, got %s", domain.ProtocolShadowsocks, n.Protocol)
	}
	if n.Security == nil {
		t.Fatalf("expected security to be set")
	}
	if n.Security.Plugin != "obfs-local" {
		t.Fatalf("expected plugin=obfs-local, got %q", n.Security.Plugin)
	}
	if n.Security.PluginOpts != "obfs=tls;obfs-host=obfs.example.com" {
		t.Fatalf("expected pluginOpts normalized, got %q", n.Security.PluginOpts)
	}
}
