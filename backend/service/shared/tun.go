package shared

import (
	"io"
	"net/http"
	"strings"
	"time"
)

// GetIPGeo 获取当前 IP 地理位置信息
func GetIPGeo() (map[string]interface{}, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://ipv4.ping0.cc/geo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析响应：IP\n位置\nASN\nISP
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	result := map[string]interface{}{
		"ip":       "",
		"location": "",
		"asn":      "",
		"isp":      "",
	}

	if len(lines) >= 1 {
		result["ip"] = strings.TrimSpace(lines[0])
	}
	if len(lines) >= 2 {
		result["location"] = strings.TrimSpace(lines[1])
	}
	if len(lines) >= 3 {
		result["asn"] = strings.TrimSpace(lines[2])
	}
	if len(lines) >= 4 {
		result["isp"] = strings.TrimSpace(lines[3])
	}

	return result, nil
}
