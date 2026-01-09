package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ipGeoResult struct {
	ip       string
	location string
	asn      string
	isp      string
}

func (r ipGeoResult) toMap() map[string]interface{} {
	return map[string]interface{}{
		"ip":       strings.TrimSpace(r.ip),
		"location": strings.TrimSpace(r.location),
		"asn":      strings.TrimSpace(r.asn),
		"isp":      strings.TrimSpace(r.isp),
	}
}

type ipGeoProvider struct {
	name  string
	url   string
	parse func([]byte) (ipGeoResult, error)
}

// GetIPGeo 获取当前 IP 地理位置信息
func GetIPGeo() (map[string]interface{}, error) {
	return GetIPGeoWithHTTPClient(nil)
}

// GetIPGeoWithHTTPClient 使用指定 HTTP Client 获取当前 IP 地理位置信息。
//
// 说明：当上层希望“强制走本地入站代理”时，需要传入带自定义 Transport 的 client。
func GetIPGeoWithHTTPClient(client *http.Client) (map[string]interface{}, error) {
	if client == nil {
		client = &http.Client{Timeout: 6 * time.Second}
	}
	providers := []ipGeoProvider{
		{name: "ping0-https", url: "https://ipv4.ping0.cc/geo", parse: parsePing0Geo},
		{name: "ping0-http", url: "http://ipv4.ping0.cc/geo", parse: parsePing0Geo},
		{name: "ipinfo", url: "https://ipinfo.io/json", parse: parseIPInfoGeo},
		{name: "ipify", url: "https://api.ipify.org?format=json", parse: parseIPify},
	}
	return getIPGeoWithClient(client, providers)
}

func getIPGeoWithClient(client *http.Client, providers []ipGeoProvider) (map[string]interface{}, error) {
	if client == nil {
		client = &http.Client{Timeout: 6 * time.Second}
	}
	if len(providers) == 0 {
		return nil, errors.New("no ip geo providers configured")
	}

	var lastErr error
	for _, p := range providers {
		if strings.TrimSpace(p.url) == "" || p.parse == nil {
			continue
		}

		result, err := fetchAndParseIPGeo(client, p)
		if err == nil {
			return result.toMap(), nil
		}
		lastErr = fmt.Errorf("%s: %w", p.name, err)
	}

	if lastErr == nil {
		lastErr = errors.New("no ip geo candidate successful")
	}
	return nil, lastErr
}

func fetchAndParseIPGeo(client *http.Client, provider ipGeoProvider) (ipGeoResult, error) {
	req, err := http.NewRequest(http.MethodGet, provider.url, nil)
	if err != nil {
		return ipGeoResult{}, err
	}
	req.Header.Set("User-Agent", "VeaIPGeo/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return ipGeoResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 读取少量 body 作为提示（防止返回超大内容）。
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		hint := strings.TrimSpace(string(b))
		if hint != "" {
			return ipGeoResult{}, fmt.Errorf("http %s: %s", resp.Status, hint)
		}
		return ipGeoResult{}, fmt.Errorf("http %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return ipGeoResult{}, err
	}

	result, err := provider.parse(body)
	if err != nil {
		return ipGeoResult{}, err
	}
	if strings.TrimSpace(result.ip) == "" {
		return ipGeoResult{}, errors.New("empty ip")
	}
	return result, nil
}

func parsePing0Geo(body []byte) (ipGeoResult, error) {
	// ping0 响应格式：IP\n位置\nASN\nISP
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) == 0 {
		return ipGeoResult{}, errors.New("empty response")
	}
	res := ipGeoResult{}
	if len(lines) >= 1 {
		res.ip = strings.TrimSpace(lines[0])
	}
	if len(lines) >= 2 {
		res.location = strings.TrimSpace(lines[1])
	}
	if len(lines) >= 3 {
		res.asn = strings.TrimSpace(lines[2])
	}
	if len(lines) >= 4 {
		res.isp = strings.TrimSpace(lines[3])
	}
	if strings.TrimSpace(res.ip) == "" {
		return ipGeoResult{}, errors.New("missing ip")
	}
	return res, nil
}

func parseIPInfoGeo(body []byte) (ipGeoResult, error) {
	var payload struct {
		IP      string `json:"ip"`
		City    string `json:"city"`
		Region  string `json:"region"`
		Country string `json:"country"`
		Org     string `json:"org"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ipGeoResult{}, err
	}

	locParts := make([]string, 0, 3)
	if strings.TrimSpace(payload.City) != "" {
		locParts = append(locParts, strings.TrimSpace(payload.City))
	}
	if strings.TrimSpace(payload.Region) != "" {
		locParts = append(locParts, strings.TrimSpace(payload.Region))
	}
	if strings.TrimSpace(payload.Country) != "" {
		locParts = append(locParts, strings.TrimSpace(payload.Country))
	}
	location := strings.Join(locParts, " ")

	asn := ""
	isp := strings.TrimSpace(payload.Org)
	if strings.HasPrefix(strings.ToUpper(isp), "AS") {
		fields := strings.Fields(isp)
		if len(fields) >= 1 {
			asn = fields[0]
		}
		if len(fields) >= 2 {
			isp = strings.Join(fields[1:], " ")
		}
	}

	return ipGeoResult{
		ip:       strings.TrimSpace(payload.IP),
		location: location,
		asn:      asn,
		isp:      isp,
	}, nil
}

func parseIPify(body []byte) (ipGeoResult, error) {
	var payload struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ipGeoResult{}, err
	}
	return ipGeoResult{ip: strings.TrimSpace(payload.IP)}, nil
}
