package shared

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetIPGeoWithClient_FallbackOnHTTPStatus(t *testing.T) {
	t.Parallel()

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	t.Cleanup(bad.Close)

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ip":"1.2.3.4"}`)
	}))
	t.Cleanup(good.Close)

	client := &http.Client{Timeout: time.Second}
	providers := []ipGeoProvider{
		{name: "bad", url: bad.URL, parse: parsePing0Geo},
		{name: "good", url: good.URL, parse: parseIPify},
	}

	got, err := getIPGeoWithClient(client, providers)
	if err != nil {
		t.Fatalf("getIPGeoWithClient: %v", err)
	}
	if got["ip"] != "1.2.3.4" {
		t.Fatalf("expected ip %q, got %v", "1.2.3.4", got["ip"])
	}
}

func TestGetIPGeoWithClient_FallbackOnParseError(t *testing.T) {
	t.Parallel()

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not-json")
	}))
	t.Cleanup(bad.Close)

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "8.8.8.8\nUS\nAS15169\nGoogle")
	}))
	t.Cleanup(good.Close)

	client := &http.Client{Timeout: time.Second}
	providers := []ipGeoProvider{
		{name: "bad", url: bad.URL, parse: parseIPify},
		{name: "good", url: good.URL, parse: parsePing0Geo},
	}

	got, err := getIPGeoWithClient(client, providers)
	if err != nil {
		t.Fatalf("getIPGeoWithClient: %v", err)
	}
	if got["ip"] != "8.8.8.8" {
		t.Fatalf("expected ip %q, got %v", "8.8.8.8", got["ip"])
	}
	if got["location"] != "US" {
		t.Fatalf("expected location %q, got %v", "US", got["location"])
	}
	if got["asn"] != "AS15169" {
		t.Fatalf("expected asn %q, got %v", "AS15169", got["asn"])
	}
	if got["isp"] != "Google" {
		t.Fatalf("expected isp %q, got %v", "Google", got["isp"])
	}
}
