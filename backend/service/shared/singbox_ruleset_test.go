package shared

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSingBoxRuleSetURL(t *testing.T) {
	t.Run("geosite", func(t *testing.T) {
		got, err := singBoxRuleSetURL("geosite-cn")
		if err != nil {
			t.Fatalf("expected nil err, got %v", err)
		}
		want := "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("geoip", func(t *testing.T) {
		got, err := singBoxRuleSetURL("geoip-cn")
		if err != nil {
			t.Fatalf("expected nil err, got %v", err)
		}
		want := "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		if _, err := singBoxRuleSetURL("cn"); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestEnsureSingBoxRuleSets_DownloadsMissing(t *testing.T) {
	root := t.TempDir()

	var gotURLs []string
	download := func(url string) ([]byte, error) {
		gotURLs = append(gotURLs, url)
		return []byte("ok"), nil
	}

	tags := []string{"geosite-cn", "geoip-cn"}
	if err := ensureSingBoxRuleSets(root, tags, download); err != nil {
		t.Fatalf("ensureSingBoxRuleSets failed: %v", err)
	}

	// 文件应落在 core/sing-box/rule-set/<tag>.srs
	for _, tag := range tags {
		p := filepath.Join(root, "core", "sing-box", "rule-set", tag+".srs")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("expected file %s: %v", p, err)
		}
		if string(b) != "ok" {
			t.Fatalf("expected file content 'ok', got %q", string(b))
		}
	}

	if len(gotURLs) != 2 {
		t.Fatalf("expected 2 downloads, got %d", len(gotURLs))
	}
}

func TestEnsureSingBoxRuleSets_SkipsExisting(t *testing.T) {
	root := t.TempDir()

	// 预写入一个非空文件
	existing := filepath.Join(root, "core", "sing-box", "rule-set", "geosite-cn.srs")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(existing, []byte("already"), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	called := 0
	download := func(url string) ([]byte, error) {
		called++
		return []byte("new"), nil
	}

	if err := ensureSingBoxRuleSets(root, []string{"geosite-cn"}, download); err != nil {
		t.Fatalf("ensureSingBoxRuleSets failed: %v", err)
	}
	if called != 0 {
		t.Fatalf("expected no download call, got %d", called)
	}
}

func TestExtractSingBoxRuleSetTagsFromConfig(t *testing.T) {
	input := []byte(`{
  "route": {
    "rule_set": [
      {"tag": "geosite-cn", "type": "local"},
      {"tag": "geosite-openai"},
      {"tag": "custom"},
      {"tag": "geoip-cn"},
      {"tag": "geosite-cn"}
    ]
  }
}`)

	got, err := ExtractSingBoxRuleSetTagsFromConfig(input)
	if err != nil {
		t.Fatalf("ExtractSingBoxRuleSetTagsFromConfig failed: %v", err)
	}

	want := []string{"geosite-cn", "geosite-openai", "geoip-cn"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}
