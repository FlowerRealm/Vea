package shared

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
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

	got, err := getIPGeoWithClient(context.Background(), client, providers)
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

	got, err := getIPGeoWithClient(context.Background(), client, providers)
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

func TestGetIPGeoWithClient_ThroughHTTPProxy(t *testing.T) {
	t.Parallel()

	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		_, _ = io.WriteString(w, `{"ip":"9.9.9.9"}`)
	}))
	t.Cleanup(proxy.Close)

	u, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = http.ProxyURL(u)
	client := &http.Client{Timeout: time.Second, Transport: tr}

	providers := []ipGeoProvider{
		{name: "proxy", url: "http://unreachable.invalid/geo", parse: parseIPify},
	}

	got, err := getIPGeoWithClient(context.Background(), client, providers)
	if err != nil {
		t.Fatalf("getIPGeoWithClient: %v", err)
	}
	if got["ip"] != "9.9.9.9" {
		t.Fatalf("expected ip %q, got %v", "9.9.9.9", got["ip"])
	}
	if proxyHits.Load() == 0 {
		t.Fatalf("expected request through http proxy, got 0 hits")
	}
}

func TestGetIPGeoWithClient_ThroughSOCKS5_NoAuth(t *testing.T) {
	t.Parallel()

	addr, stats := startTestSOCKS5Server(t, testSOCKS5ServerOpts{})

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return DialSOCKS5Context(ctx, addr, nil, network, address)
	}
	client := &http.Client{Timeout: time.Second, Transport: tr}

	providers := []ipGeoProvider{
		{name: "socks5", url: "http://unreachable.invalid/geo", parse: parseIPify},
	}

	got, err := getIPGeoWithClient(context.Background(), client, providers)
	if err != nil {
		t.Fatalf("getIPGeoWithClient: %v", err)
	}
	if got["ip"] != "1.2.3.4" {
		t.Fatalf("expected ip %q, got %v", "1.2.3.4", got["ip"])
	}

	destHost, destPort, sawAuth := stats()
	if destHost != "unreachable.invalid" || destPort != 80 {
		t.Fatalf("expected dest unreachable.invalid:80, got %s:%d", destHost, destPort)
	}
	if sawAuth {
		t.Fatalf("expected no auth, got auth negotiation")
	}
}

func TestGetIPGeoWithClient_ThroughSOCKS5_UserPass(t *testing.T) {
	t.Parallel()

	auth := &SOCKS5Auth{Username: "u", Password: "p"}
	addr, stats := startTestSOCKS5Server(t, testSOCKS5ServerOpts{RequireAuth: true, Auth: auth})

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return DialSOCKS5Context(ctx, addr, auth, network, address)
	}
	client := &http.Client{Timeout: time.Second, Transport: tr}

	providers := []ipGeoProvider{
		{name: "socks5", url: "http://unreachable.invalid/geo", parse: parseIPify},
	}

	got, err := getIPGeoWithClient(context.Background(), client, providers)
	if err != nil {
		t.Fatalf("getIPGeoWithClient: %v", err)
	}
	if got["ip"] != "1.2.3.4" {
		t.Fatalf("expected ip %q, got %v", "1.2.3.4", got["ip"])
	}

	destHost, destPort, sawAuth := stats()
	if destHost != "unreachable.invalid" || destPort != 80 {
		t.Fatalf("expected dest unreachable.invalid:80, got %s:%d", destHost, destPort)
	}
	if !sawAuth {
		t.Fatalf("expected auth negotiation, got none")
	}
}

type testSOCKS5ServerOpts struct {
	RequireAuth bool
	Auth        *SOCKS5Auth
}

func startTestSOCKS5Server(t *testing.T, opts testSOCKS5ServerOpts) (addr string, stats func() (destHost string, destPort int, sawAuth bool)) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	var destHostVal atomic.Value
	var destPortVal atomic.Int32
	var sawAuthVal atomic.Bool

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go handleTestSOCKS5Conn(c, opts, &destHostVal, &destPortVal, &sawAuthVal)
		}
	}()

	return ln.Addr().String(), func() (string, int, bool) {
		host, _ := destHostVal.Load().(string)
		return host, int(destPortVal.Load()), sawAuthVal.Load()
	}
}

func handleTestSOCKS5Conn(c net.Conn, opts testSOCKS5ServerOpts, destHost *atomic.Value, destPort *atomic.Int32, sawAuth *atomic.Bool) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))

	br := bufio.NewReader(c)

	// Greeting
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(br, hdr); err != nil {
		return
	}
	if hdr[0] != 0x05 {
		return
	}
	nMethods := int(hdr[1])
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(br, methods); err != nil {
		return
	}

	hasNoAuth := bytesContains(methods, 0x00)
	hasUserPass := bytesContains(methods, 0x02)

	method := byte(0xff)
	if opts.RequireAuth {
		if hasUserPass {
			method = 0x02
		}
	} else if hasNoAuth {
		method = 0x00
	} else if hasUserPass {
		method = 0x02
	}

	if _, err := c.Write([]byte{0x05, method}); err != nil {
		return
	}
	if method == 0xff {
		return
	}

	// Username/password auth
	if method == 0x02 {
		sawAuth.Store(true)

		abuf := make([]byte, 2)
		if _, err := io.ReadFull(br, abuf); err != nil {
			return
		}
		if abuf[0] != 0x01 {
			return
		}
		ulen := int(abuf[1])
		ub := make([]byte, ulen)
		if _, err := io.ReadFull(br, ub); err != nil {
			return
		}
		pb := make([]byte, 1)
		if _, err := io.ReadFull(br, pb); err != nil {
			return
		}
		plen := int(pb[0])
		pwd := make([]byte, plen)
		if _, err := io.ReadFull(br, pwd); err != nil {
			return
		}

		ok := opts.Auth != nil && string(ub) == opts.Auth.Username && string(pwd) == opts.Auth.Password
		status := byte(0x01)
		if ok {
			status = 0x00
		}
		_, _ = c.Write([]byte{0x01, status})
		if !ok {
			return
		}
	}

	// CONNECT request
	reqHdr := make([]byte, 4)
	if _, err := io.ReadFull(br, reqHdr); err != nil {
		return
	}
	if reqHdr[0] != 0x05 || reqHdr[1] != 0x01 {
		return
	}

	var host string
	switch reqHdr[3] {
	case 0x01: // IPv4
		ip := make([]byte, 4)
		if _, err := io.ReadFull(br, ip); err != nil {
			return
		}
		host = net.IP(ip).String()
	case 0x03: // Domain
		lb := make([]byte, 1)
		if _, err := io.ReadFull(br, lb); err != nil {
			return
		}
		l := int(lb[0])
		b := make([]byte, l)
		if _, err := io.ReadFull(br, b); err != nil {
			return
		}
		host = string(b)
	case 0x04: // IPv6
		ip := make([]byte, 16)
		if _, err := io.ReadFull(br, ip); err != nil {
			return
		}
		host = net.IP(ip).String()
	default:
		return
	}

	pb := make([]byte, 2)
	if _, err := io.ReadFull(br, pb); err != nil {
		return
	}
	port := int(pb[0])<<8 | int(pb[1])

	destHost.Store(host)
	destPort.Store(int32(port))

	// CONNECT success
	_, _ = c.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	// Respond like an origin server (no actual forwarding in test).
	_, _ = io.WriteString(c, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 16\r\nConnection: close\r\n\r\n{\"ip\":\"1.2.3.4\"}")
}

func bytesContains(b []byte, v byte) bool {
	for _, x := range b {
		if x == v {
			return true
		}
	}
	return false
}
