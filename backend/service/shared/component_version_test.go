package shared

import "testing"

func TestParseCoreBinaryVersion_SingBox_PrefersSingBoxVersion(t *testing.T) {
	t.Parallel()

	out := "sing-box version 1.12.12 (go1.22.0 linux/amd64)"
	if got := parseCoreBinaryVersion("singbox", out); got != "1.12.12" {
		t.Fatalf("unexpected version: %q", got)
	}
}

func TestParseCoreBinaryVersion_Clash_PrefersMihomoVersion(t *testing.T) {
	t.Parallel()

	out := "Mihomo Meta v1.18.8 (go1.22.0 windows/amd64)"
	if got := parseCoreBinaryVersion("clash", out); got != "v1.18.8" {
		t.Fatalf("unexpected version: %q", got)
	}
}

func TestParseCoreBinaryVersion_SkipsGoVersionFallback(t *testing.T) {
	t.Parallel()

	out := "Go: go1.22.0"
	if got := parseCoreBinaryVersion("", out); got != "" {
		t.Fatalf("unexpected version: %q", got)
	}
}

func TestExtractVersionFromPath(t *testing.T) {
	t.Parallel()

	path := `/tmp/sing-box-1.11.0/sing-box.exe`
	if got := extractVersionFromPath(path); got != "1.11.0" {
		t.Fatalf("unexpected version: %q", got)
	}
}

func TestNormalizeVersionTag(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"1.2.3":  "v1.2.3",
		"v1.2.3": "v1.2.3",
		"V1.2.3": "v1.2.3",
	}
	for in, want := range cases {
		if got := normalizeVersionTag(in); got != want {
			t.Fatalf("normalizeVersionTag(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestDetectCoreBinaryVersion_UsesPathWhenPresent(t *testing.T) {
	t.Parallel()

	version, err := DetectCoreBinaryVersion("singbox", "/tmp/sing-box-1.9.0/sing-box")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "v1.9.0" {
		t.Fatalf("unexpected version: %q", version)
	}
}
