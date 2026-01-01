package persist

import (
	"testing"
)

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
