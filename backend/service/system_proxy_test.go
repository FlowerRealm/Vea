package service

import (
	"os"
	"os/user"
	"reflect"
	"strings"
	"testing"

	"vea/backend/domain"
)

func TestResolveTargetUser(t *testing.T) {
	// Save original env
	origSudoUser := os.Getenv("SUDO_USER")
	origPkexecUid := os.Getenv("PKEXEC_UID")
	defer func() {
		os.Setenv("SUDO_USER", origSudoUser)
		os.Setenv("PKEXEC_UID", origPkexecUid)
	}()

	t.Run("SUDO_USER priority", func(t *testing.T) {
		os.Setenv("SUDO_USER", "testuser")
		os.Setenv("PKEXEC_UID", "1000")
		if got := resolveTargetUser(); got != "testuser" {
			t.Errorf("resolveTargetUser() = %v, want %v", got, "testuser")
		}
	})

	t.Run("PKEXEC_UID fallback", func(t *testing.T) {
		os.Unsetenv("SUDO_USER")
		// We need a valid UID for LookupId to work, usually 0 is root
		os.Setenv("PKEXEC_UID", "0")
		if got := resolveTargetUser(); got != "root" {
			t.Errorf("resolveTargetUser() = %v, want %v", got, "root")
		}
	})

	t.Run("Current user fallback", func(t *testing.T) {
		os.Unsetenv("SUDO_USER")
		os.Unsetenv("PKEXEC_UID")
		u, _ := user.Current()
		if got := resolveTargetUser(); got != u.Username {
			t.Errorf("resolveTargetUser() = %v, want %v", got, u.Username)
		}
	})
}

func TestBuildUserEnv(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("skipping test: cannot get current user")
	}

	env := buildUserEnv(u.Username)
	if env["USER"] != u.Username {
		t.Errorf("env[USER] = %v, want %v", env["USER"], u.Username)
	}
	if env["HOME"] != u.HomeDir {
		t.Errorf("env[HOME] = %v, want %v", env["HOME"], u.HomeDir)
	}
}

func TestNormalizeSystemProxySettings(t *testing.T) {
	tests := []struct {
		name     string
		input    domain.SystemProxySettings
		wantHost []string // expected ignore hosts (sorted)
	}{
		{
			name: "Empty",
			input: domain.SystemProxySettings{
				Enabled:     true,
				IgnoreHosts: []string{},
			},
			wantHost: []string{
				"*.local", "10.0.0.0/8", "127.0.0.1", "172.16.0.0/12", "192.168.0.0/16", "::1", "localhost",
			},
		},
		{
			name: "With Duplicates and Case",
			input: domain.SystemProxySettings{
				Enabled:     true,
				IgnoreHosts: []string{"Localhost", "example.com", "EXAMPLE.COM", ""},
			},
			wantHost: []string{
				"*.local", "10.0.0.0/8", "127.0.0.1", "172.16.0.0/12", "192.168.0.0/16", "::1", "example.com", "localhost",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSystemProxySettings(tt.input)
			if !reflect.DeepEqual(got.IgnoreHosts, tt.wantHost) {
				t.Errorf("normalizeSystemProxySettings().IgnoreHosts = %v, want %v", got.IgnoreHosts, tt.wantHost)
			}
		})
	}
}

func TestMergeEnv(t *testing.T) {
	base := []string{"A=1", "B=2"}
	extra := map[string]string{"B": "3", "C": "4"}
	got := mergeEnv(base, extra)

	gotMap := make(map[string]string)
	for _, kv := range got {
		// simple split, assuming no = in value for this test
		// real implementation handles it, but for test verification simple split is enough if values are simple
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			gotMap[parts[0]] = parts[1]
		}
	}

	if gotMap["A"] != "1" {
		t.Errorf("want A=1, got %v", gotMap["A"])
	}
	if gotMap["B"] != "3" {
		t.Errorf("want B=3, got %v", gotMap["B"])
	}
	if gotMap["C"] != "4" {
		t.Errorf("want C=4, got %v", gotMap["C"])
	}
}

func TestNormalizeGSettingsValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"'manual'", "manual"},
		{"'manual'\n", "manual"},
		{"uint32 123", "123"},
		{"'127.0.0.1'", "127.0.0.1"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeGSettingsValue(tt.input); got != tt.want {
			t.Errorf("normalizeGSettingsValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

