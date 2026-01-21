package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"vea/backend/domain"
	"vea/backend/repository/events"
	"vea/backend/repository/memory"
	"vea/backend/service/nodes"
)

func TestService_Create_WithSourceURL_SyncsInBackground(t *testing.T) {
	t.Parallel()

	const payload = "hello"
	var gotUserAgent atomic.Value
	var requestOnce sync.Once
	requested := make(chan struct{})
	unblock := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent.Store(r.UserAgent())
		requestOnce.Do(func() { close(requested) })
		<-unblock
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	repo := memory.NewConfigRepo(memory.NewStore(events.NewBus()))
	svc := NewService(context.Background(), repo, nil, nil)

	var (
		created domain.Config
		err     error
	)

	done := make(chan struct{})
	go func() {
		created, err = svc.Create(context.Background(), domain.Config{
			Name:      "cfg-1",
			Format:    domain.ConfigFormatSubscription,
			SourceURL: srv.URL,
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("expected Create to return without waiting for remote payload download")
	}
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if created.Payload != "" {
		t.Fatalf("expected payload to be empty before background sync, got %q", created.Payload)
	}

	select {
	case <-requested:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected background sync to start download request")
	}
	if got, ok := gotUserAgent.Load().(string); !ok || got != subscriptionUserAgent {
		t.Fatalf("expected User-Agent %q, got %q", subscriptionUserAgent, got)
	}

	close(unblock)

	sum := sha256.Sum256([]byte(payload))
	expectedChecksum := hex.EncodeToString(sum[:])

	waitUntil(t, 3*time.Second, func() bool {
		got, getErr := repo.Get(context.Background(), created.ID)
		if getErr != nil {
			return false
		}
		if got.Payload != payload {
			return false
		}
		if got.Checksum != expectedChecksum {
			return false
		}
		if got.LastSyncedAt.IsZero() {
			return false
		}
		return got.LastSyncError == ""
	})
}

func TestService_Create_WithSourceURL_FallbackPayload_ClearsSyncError(t *testing.T) {
	t.Parallel()

	const fallbackPayload = "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls#n1"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, nil)

	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
		Payload:   fallbackPayload,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	sum := sha256.Sum256([]byte(fallbackPayload))
	expectedChecksum := hex.EncodeToString(sum[:])

	waitUntil(t, 3*time.Second, func() bool {
		updated, getErr := configRepo.Get(context.Background(), created.ID)
		if getErr != nil {
			return false
		}
		if updated.Payload != fallbackPayload {
			return false
		}
		if updated.Checksum != expectedChecksum {
			return false
		}
		if updated.LastSyncError != "" {
			return false
		}

		createdNodes, listErr := nodeRepo.ListByConfigID(context.Background(), created.ID)
		return listErr == nil && len(createdNodes) == 1
	})
}

func TestService_Sync_UnchangedChecksum_OnlyUpdatesLastSyncedAt(t *testing.T) {
	t.Parallel()

	const payload = "same-content"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	repo := memory.NewConfigRepo(memory.NewStore(events.NewBus()))
	svc := NewService(context.Background(), repo, nil, nil)

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
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, frouterRepo)

	payload.Store("vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls#n1")
	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	waitUntil(t, 3*time.Second, func() bool {
		nodesBefore, listErr := nodeRepo.ListByConfigID(context.Background(), created.ID)
		return listErr == nil && len(nodesBefore) == 1
	})

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
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, frouterRepo)

	payload.Store("vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls#n1")
	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	waitUntil(t, 3*time.Second, func() bool {
		nodesBefore, listErr := nodeRepo.ListByConfigID(context.Background(), created.ID)
		return listErr == nil && len(nodesBefore) == 1
	})

	cfgBefore, err := configRepo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get config before sync: %v", err)
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
	if updatedCfg.Payload != cfgBefore.Payload {
		t.Fatalf("expected payload preserved on empty sync, got %q vs %q", updatedCfg.Payload, cfgBefore.Payload)
	}
	if updatedCfg.Checksum != cfgBefore.Checksum {
		t.Fatalf("expected checksum preserved on empty sync, got %q vs %q", updatedCfg.Checksum, cfgBefore.Checksum)
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
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, frouterRepo)

	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	frouterID := stableFRouterIDForConfig(created.ID)
	waitUntil(t, 3*time.Second, func() bool {
		createdNodes, listErr := nodeRepo.ListByConfigID(context.Background(), created.ID)
		if listErr != nil || len(createdNodes) != 1 {
			return false
		}
		_, getErr := frouterRepo.Get(context.Background(), frouterID)
		return getErr == nil
	})

	createdNodes, err := nodeRepo.ListByConfigID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if createdNodes[0].Protocol != domain.ProtocolVMess {
		t.Fatalf("expected protocol=%s, got %s", domain.ProtocolVMess, createdNodes[0].Protocol)
	}

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
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, frouterRepo)

	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-compact",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	frouterID := stableFRouterIDForConfig(created.ID)
	waitUntil(t, 3*time.Second, func() bool {
		createdNodes, listErr := nodeRepo.ListByConfigID(context.Background(), created.ID)
		if listErr != nil || len(createdNodes) != 1 {
			return false
		}
		_, getErr := frouterRepo.Get(context.Background(), frouterID)
		return getErr == nil
	})

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
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, frouterRepo)

	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-ss-obfs",
		Format:    domain.ConfigFormatSubscription,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	waitUntil(t, 3*time.Second, func() bool {
		createdNodes, listErr := nodeRepo.ListByConfigID(context.Background(), created.ID)
		return listErr == nil && len(createdNodes) == 1
	})

	createdNodes, err := nodeRepo.ListByConfigID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
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

func TestService_SyncNodesFromPayload_ReusesExistingNodeID_PreservesFRouterReferences(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	frouterRepo := memory.NewFRouterRepo(store)
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, frouterRepo)

	const configID = "cfg-1"
	if _, err := configRepo.Create(ctx, domain.Config{
		ID:     configID,
		Name:   "cfg-1",
		Format: domain.ConfigFormatSubscription,
	}); err != nil {
		t.Fatalf("create config: %v", err)
	}

	const existingNodeID = "old-node-id"
	if _, err := nodeRepo.Create(ctx, domain.Node{
		ID:             existingNodeID,
		Name:           "n1",
		Address:        "example.com",
		Port:           443,
		Protocol:       domain.ProtocolVLESS,
		SourceConfigID: configID,
		Security: &domain.NodeSecurity{
			UUID: "11111111-1111-1111-1111-111111111111",
		},
		Transport: &domain.NodeTransport{
			Type: "tcp",
		},
		TLS: &domain.NodeTLS{
			Enabled: true,
			Type:    "tls",
		},
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}

	if _, err := frouterRepo.Create(ctx, domain.FRouter{
		ID:   "fr-1",
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Slots: []domain.SlotNode{
				{ID: "slot-1", Name: "slot", BoundNodeID: existingNodeID},
			},
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "slot-1", Priority: 0, Enabled: true},
			},
		},
	}); err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	const payload = "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls#n1"
	if err := svc.syncNodesFromPayload(ctx, configID, payload); err != nil {
		t.Fatalf("sync nodes: %v", err)
	}

	if _, err := nodeRepo.Get(ctx, existingNodeID); err != nil {
		t.Fatalf("expected node %s to be preserved, got err=%v", existingNodeID, err)
	}
}

func TestService_SyncNodesFromPayload_ShareLinks_ReusesExistingNodeID_BySourceKey_WhenUUIDChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	frouterRepo := memory.NewFRouterRepo(store)
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, frouterRepo)

	const configID = "cfg-1"
	if _, err := configRepo.Create(ctx, domain.Config{
		ID:     configID,
		Name:   "cfg-1",
		Format: domain.ConfigFormatSubscription,
	}); err != nil {
		t.Fatalf("create config: %v", err)
	}

	const existingNodeID = "old-node-id"
	if _, err := nodeRepo.Create(ctx, domain.Node{
		ID:             existingNodeID,
		Name:           "custom-name",
		SourceKey:      "n1",
		Address:        "example.com",
		Port:           443,
		Protocol:       domain.ProtocolVLESS,
		SourceConfigID: configID,
		Security: &domain.NodeSecurity{
			UUID: "old-uuid",
		},
		Transport: &domain.NodeTransport{
			Type: "tcp",
		},
		TLS: &domain.NodeTLS{
			Enabled: true,
			Type:    "tls",
		},
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}

	if _, err := frouterRepo.Create(ctx, domain.FRouter{
		ID:   "fr-1",
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Slots: []domain.SlotNode{
				{ID: "slot-1", Name: "slot", BoundNodeID: existingNodeID},
			},
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "slot-1", Priority: 0, Enabled: true},
			},
		},
	}); err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	const payload = "vless://new-uuid@example.com:443?security=tls#n1"
	if err := svc.syncNodesFromPayload(ctx, configID, payload); err != nil {
		t.Fatalf("sync nodes: %v", err)
	}

	n, err := nodeRepo.Get(ctx, existingNodeID)
	if err != nil {
		t.Fatalf("expected node %s to be preserved, got err=%v", existingNodeID, err)
	}
	if n.Name != "custom-name" {
		t.Fatalf("expected node name preserved=%q, got %q", "custom-name", n.Name)
	}
	if n.SourceKey != "n1" {
		t.Fatalf("expected node sourceKey=%q, got %q", "n1", n.SourceKey)
	}
	if n.Security == nil || n.Security.UUID != "new-uuid" {
		t.Fatalf("expected node uuid updated=%q, got %+v", "new-uuid", n.Security)
	}
}

func TestService_SyncNodesFromPayload_ShareLinks_LegacyNode_ReusesExistingNodeID_ByNameKey_WhenUUIDChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	frouterRepo := memory.NewFRouterRepo(store)
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, frouterRepo)

	const configID = "cfg-1"
	if _, err := configRepo.Create(ctx, domain.Config{
		ID:     configID,
		Name:   "cfg-1",
		Format: domain.ConfigFormatSubscription,
	}); err != nil {
		t.Fatalf("create config: %v", err)
	}

	const existingNodeID = "old-node-id"
	if _, err := nodeRepo.Create(ctx, domain.Node{
		ID:             existingNodeID,
		Name:           "n1",
		Address:        "example.com",
		Port:           443,
		Protocol:       domain.ProtocolVLESS,
		SourceConfigID: configID,
		Security: &domain.NodeSecurity{
			UUID: "old-uuid",
		},
		Transport: &domain.NodeTransport{
			Type: "tcp",
		},
		TLS: &domain.NodeTLS{
			Enabled: true,
			Type:    "tls",
		},
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}

	const payload = "vless://new-uuid@example.com:443?security=tls#n1"
	if err := svc.syncNodesFromPayload(ctx, configID, payload); err != nil {
		t.Fatalf("sync nodes: %v", err)
	}

	n, err := nodeRepo.Get(ctx, existingNodeID)
	if err != nil {
		t.Fatalf("expected node %s to be preserved, got err=%v", existingNodeID, err)
	}
	if n.SourceKey != "n1" {
		t.Fatalf("expected node sourceKey backfilled=%q, got %q", "n1", n.SourceKey)
	}
	if n.Security == nil || n.Security.UUID != "new-uuid" {
		t.Fatalf("expected node uuid updated=%q, got %+v", "new-uuid", n.Security)
	}
}

func TestService_SyncNodesFromPayload_ClashYAML_ReusesExistingNodeID_RewritesChainProxy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	frouterRepo := memory.NewFRouterRepo(store)
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, frouterRepo)

	const configID = "cfg-1"
	if _, err := configRepo.Create(ctx, domain.Config{
		ID:     configID,
		Name:   "cfg-1",
		Format: domain.ConfigFormatSubscription,
	}); err != nil {
		t.Fatalf("create config: %v", err)
	}

	const existingNodeID = "old-node-id"
	if _, err := nodeRepo.Create(ctx, domain.Node{
		ID:             existingNodeID,
		Name:           "n1",
		Address:        "example.com",
		Port:           443,
		Protocol:       domain.ProtocolVMess,
		SourceConfigID: configID,
		Security: &domain.NodeSecurity{
			UUID:       "11111111-1111-1111-1111-111111111111",
			AlterID:    0,
			Encryption: "auto",
		},
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}

	const payload = `
proxies:
  - name: n1
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
rules:
  - MATCH,n1
`

	if err := svc.syncNodesFromPayload(ctx, configID, payload); err != nil {
		t.Fatalf("sync nodes: %v", err)
	}

	nodesAfter, err := nodeRepo.ListByConfigID(ctx, configID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodesAfter) != 1 {
		t.Fatalf("expected nodes=1 after sync, got %d", len(nodesAfter))
	}
	if nodesAfter[0].ID != existingNodeID {
		t.Fatalf("expected node id preserved=%s, got %s", existingNodeID, nodesAfter[0].ID)
	}

	frouterID := stableFRouterIDForConfig(configID)
	frouter, err := frouterRepo.Get(ctx, frouterID)
	if err != nil {
		t.Fatalf("get frouter: %v", err)
	}
	if len(frouter.ChainProxy.Edges) == 0 {
		t.Fatalf("expected subscription frouter to have edges")
	}

	found := false
	for _, edge := range frouter.ChainProxy.Edges {
		if edge.From == domain.EdgeNodeLocal && edge.To == existingNodeID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected frouter edges to reference preserved node id %s, got edges=%v", existingNodeID, frouter.ChainProxy.Edges)
	}
}

func TestService_SyncNodesFromPayload_ClashYAML_ReusesExistingNodeID_WhenTransportDetailsChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	frouterRepo := memory.NewFRouterRepo(store)
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, frouterRepo)

	const configID = "cfg-1"
	if _, err := configRepo.Create(ctx, domain.Config{
		ID:     configID,
		Name:   "cfg-1",
		Format: domain.ConfigFormatSubscription,
	}); err != nil {
		t.Fatalf("create config: %v", err)
	}

	const existingNodeID = "old-node-id"
	if _, err := nodeRepo.Create(ctx, domain.Node{
		ID:             existingNodeID,
		Name:           "n1",
		Address:        "example.com",
		Port:           443,
		Protocol:       domain.ProtocolVMess,
		SourceConfigID: configID,
		Security: &domain.NodeSecurity{
			UUID:       "11111111-1111-1111-1111-111111111111",
			AlterID:    0,
			Encryption: "auto",
		},
		Transport: &domain.NodeTransport{
			Type: "ws",
			Path: "/old",
		},
		TLS: &domain.NodeTLS{
			Enabled:     true,
			Type:        "tls",
			Fingerprint: "chrome",
		},
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}

	const payload = `
proxies:
  - name: n1
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    tls: true
    client-fingerprint: safari
    network: ws
    ws-opts:
      path: /new
rules:
  - MATCH,n1
`

	if err := svc.syncNodesFromPayload(ctx, configID, payload); err != nil {
		t.Fatalf("sync nodes: %v", err)
	}

	nodesAfter, err := nodeRepo.ListByConfigID(ctx, configID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodesAfter) != 1 {
		t.Fatalf("expected nodes=1 after sync, got %d", len(nodesAfter))
	}
	if nodesAfter[0].ID != existingNodeID {
		t.Fatalf("expected node id preserved=%s, got %s", existingNodeID, nodesAfter[0].ID)
	}

	n, err := nodeRepo.Get(ctx, existingNodeID)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if n.Transport == nil || n.Transport.Type != "ws" || n.Transport.Path != "/new" {
		t.Fatalf("expected transport updated to ws path /new, got transport=%v", n.Transport)
	}
	if n.TLS == nil || !n.TLS.Enabled || strings.ToLower(strings.TrimSpace(n.TLS.Fingerprint)) != "safari" {
		t.Fatalf("expected tls fingerprint updated to safari, got tls=%v", n.TLS)
	}
}

func TestService_SyncNodesFromPayload_ClashYAML_ReusesExistingNodeID_WhenIdentityConflicts_UsesNameToDisambiguate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	frouterRepo := memory.NewFRouterRepo(store)
	nodeSvc := nodes.NewService(context.Background(), nodeRepo)
	svc := NewService(context.Background(), configRepo, nodeSvc, frouterRepo)

	const configID = "cfg-1"
	if _, err := configRepo.Create(ctx, domain.Config{
		ID:     configID,
		Name:   "cfg-1",
		Format: domain.ConfigFormatSubscription,
	}); err != nil {
		t.Fatalf("create config: %v", err)
	}

	const (
		nodeAID = "node-a-id"
		nodeBID = "node-b-id"
		uuidID  = "11111111-1111-1111-1111-111111111111"
	)

	if _, err := nodeRepo.Create(ctx, domain.Node{
		ID:             nodeAID,
		Name:           "A",
		Address:        "example.com",
		Port:           443,
		Protocol:       domain.ProtocolVMess,
		SourceConfigID: configID,
		Security: &domain.NodeSecurity{
			UUID:       uuidID,
			AlterID:    0,
			Encryption: "auto",
		},
		Transport: &domain.NodeTransport{
			Type: "ws",
			Path: "/a-old",
		},
		TLS: &domain.NodeTLS{
			Enabled:     true,
			Type:        "tls",
			Fingerprint: "chrome",
		},
	}); err != nil {
		t.Fatalf("create node a: %v", err)
	}
	if _, err := nodeRepo.Create(ctx, domain.Node{
		ID:             nodeBID,
		Name:           "B",
		Address:        "example.com",
		Port:           443,
		Protocol:       domain.ProtocolVMess,
		SourceConfigID: configID,
		Security: &domain.NodeSecurity{
			UUID:       uuidID,
			AlterID:    0,
			Encryption: "auto",
		},
		Transport: &domain.NodeTransport{
			Type: "ws",
			Path: "/b-old",
		},
		TLS: &domain.NodeTLS{
			Enabled:     true,
			Type:        "tls",
			Fingerprint: "firefox",
		},
	}); err != nil {
		t.Fatalf("create node b: %v", err)
	}

	if _, err := frouterRepo.Create(ctx, domain.FRouter{
		ID:   "fr-1",
		Name: "fr-1",
		ChainProxy: domain.ChainProxySettings{
			Slots: []domain.SlotNode{
				{ID: "slot-a", Name: "A", BoundNodeID: nodeAID},
				{ID: "slot-b", Name: "B", BoundNodeID: nodeBID},
			},
			Edges: []domain.ProxyEdge{
				{ID: "e-a", From: domain.EdgeNodeLocal, To: "slot-a", Priority: 0, Enabled: true},
				{ID: "e-b", From: domain.EdgeNodeLocal, To: "slot-b", Priority: 0, Enabled: true},
			},
		},
	}); err != nil {
		t.Fatalf("create frouter: %v", err)
	}

	const payload = `
proxies:
  - name: A
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    tls: true
    client-fingerprint: safari
    network: ws
    ws-opts:
      path: /a-new
  - name: B
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    tls: true
    client-fingerprint: chrome
    network: ws
    ws-opts:
      path: /b-new
rules:
  - MATCH,A
`

	if err := svc.syncNodesFromPayload(ctx, configID, payload); err != nil {
		t.Fatalf("sync nodes: %v", err)
	}

	nodesAfter, err := nodeRepo.ListByConfigID(ctx, configID)
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodesAfter) != 2 {
		t.Fatalf("expected nodes=2 after sync, got %d", len(nodesAfter))
	}

	if _, err := nodeRepo.Get(ctx, nodeAID); err != nil {
		t.Fatalf("expected node A id preserved=%s, got err=%v", nodeAID, err)
	}
	if _, err := nodeRepo.Get(ctx, nodeBID); err != nil {
		t.Fatalf("expected node B id preserved=%s, got err=%v", nodeBID, err)
	}

	nodeA, err := nodeRepo.Get(ctx, nodeAID)
	if err != nil {
		t.Fatalf("get node A: %v", err)
	}
	if nodeA.Transport == nil || nodeA.Transport.Type != "ws" || nodeA.Transport.Path != "/a-new" {
		t.Fatalf("expected node A transport updated to /a-new, got transport=%v", nodeA.Transport)
	}

	nodeB, err := nodeRepo.Get(ctx, nodeBID)
	if err != nil {
		t.Fatalf("get node B: %v", err)
	}
	if nodeB.Transport == nil || nodeB.Transport.Type != "ws" || nodeB.Transport.Path != "/b-new" {
		t.Fatalf("expected node B transport updated to /b-new, got transport=%v", nodeB.Transport)
	}
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}
