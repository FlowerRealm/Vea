package persist

import (
	"strings"
	"testing"
)

func TestMigrator_Migrate_EmptyReturnsDefaultState(t *testing.T) {
	t.Parallel()

	m := NewMigrator()
	state, err := m.Migrate(nil)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if state.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schemaVersion %q, got %q", SchemaVersion, state.SchemaVersion)
	}
}

func TestMigrator_Migrate_InvalidJSONReturnsError(t *testing.T) {
	t.Parallel()

	m := NewMigrator()
	if _, err := m.Migrate([]byte("{")); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestMigrator_Migrate_UnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	m := NewMigrator()
	_, err := m.Migrate([]byte(`{"schemaVersion":"9.9.9"}`))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported schemaVersion") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMigrator_Migrate_LegacyWithoutSchemaVersion(t *testing.T) {
	t.Parallel()

	data := []byte(`{
  "nodes": [
    {
      "id": "n1",
      "name": "node-1",
      "address": "example.com",
      "port": 443,
      "protocol": "vless",
      "tags": [],
      "createdAt": "2025-01-01T00:00:00Z",
      "updatedAt": "2025-01-01T00:00:00Z"
    }
  ],
  "frouters": [],
  "configs": [],
  "geoResources": [],
  "components": [],
  "systemProxy": {
    "enabled": false,
    "ignoreHosts": ["127.0.0.0/8", "::1", "localhost"],
    "updatedAt": "2025-01-01T00:00:00Z"
  },
  "proxyConfig": {
    "inboundMode": "mixed",
    "inboundPort": 1080,
    "preferredEngine": "auto",
    "frouterId": "",
    "updatedAt": "2025-01-01T00:00:00Z"
  },
  "generatedAt": "2025-01-01T00:00:00Z"
}`)

	m := NewMigrator()
	state, err := m.Migrate(data)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if state.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schemaVersion %q, got %q", SchemaVersion, state.SchemaVersion)
	}
	if len(state.Nodes) != 1 || state.Nodes[0].ID != "n1" {
		t.Fatalf("expected migrated nodes to be preserved, got %+v", state.Nodes)
	}
}

func TestMigrator_Migrate_Legacy_2_0_0_To_2_1_0(t *testing.T) {
	t.Parallel()

	data := []byte(`{
  "schemaVersion": "2.0.0",
  "nodes": [
    {
      "id": "n1",
      "name": "node-1",
      "address": "example.com",
      "port": 443,
      "protocol": "vless",
      "tags": [],
      "createdAt": "2025-01-01T00:00:00Z",
      "updatedAt": "2025-01-01T00:00:00Z"
    }
  ],
  "frouters": [
    {
      "id": "fr1",
      "name": "fr-1",
      "nodes": [
        {
          "id": "n2",
          "name": "node-2",
          "address": "example.net",
          "port": 443,
          "protocol": "trojan",
          "tags": [],
          "sourceConfigId": "",
          "createdAt": "2025-01-01T00:00:00Z",
          "updatedAt": "2025-01-01T00:00:00Z"
        }
      ],
      "chainProxy": {
        "edges": [
          {"id":"e1","from":"local","to":"direct","enabled":true}
        ]
      },
      "tags": [],
      "sourceConfigId": "cfg-1",
      "lastLatencyMs": 0,
      "lastLatencyAt": "2025-01-01T00:00:00Z",
      "lastSpeedMbps": 0,
      "lastSpeedAt": "2025-01-01T00:00:00Z",
      "createdAt": "2025-01-01T00:00:00Z",
      "updatedAt": "2025-01-01T00:00:00Z"
    }
  ],
  "configs": [],
  "geoResources": [],
  "components": [],
  "systemProxy": {
    "enabled": false,
    "ignoreHosts": ["127.0.0.0/8", "::1", "localhost"],
    "updatedAt": "2025-01-01T00:00:00Z"
  },
  "proxyConfig": {
    "inboundMode": "mixed",
    "inboundPort": 1080,
    "preferredEngine": "auto",
    "frouterId": "",
    "updatedAt": "2025-01-01T00:00:00Z"
  },
  "generatedAt": "2025-01-01T00:00:00Z"
}`)

	m := NewMigrator()
	state, err := m.Migrate(data)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if state.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schemaVersion %q, got %q", SchemaVersion, state.SchemaVersion)
	}
	if state.GeneratedAt.IsZero() {
		t.Fatalf("expected GeneratedAt to be set")
	}

	// Nodes: should include legacy global nodes + frouter embedded nodes, de-duped by id.
	nodesByID := make(map[string]struct{}, len(state.Nodes))
	var migratedN2Source string
	for _, n := range state.Nodes {
		nodesByID[n.ID] = struct{}{}
		if n.ID == "n2" {
			migratedN2Source = n.SourceConfigID
		}
	}
	if _, ok := nodesByID["n1"]; !ok {
		t.Fatalf("expected migrated nodes to contain n1")
	}
	if _, ok := nodesByID["n2"]; !ok {
		t.Fatalf("expected migrated nodes to contain n2")
	}
	if migratedN2Source != "cfg-1" {
		t.Fatalf("expected embedded node sourceConfigId inherited from frouter, got %q", migratedN2Source)
	}

	if len(state.FRouters) != 1 {
		t.Fatalf("expected 1 frouter, got %d", len(state.FRouters))
	}
	if state.FRouters[0].ID != "fr1" {
		t.Fatalf("expected frouter id fr1, got %q", state.FRouters[0].ID)
	}
	if state.FRouters[0].SourceConfigID != "cfg-1" {
		t.Fatalf("expected frouter sourceConfigId cfg-1, got %q", state.FRouters[0].SourceConfigID)
	}
}
