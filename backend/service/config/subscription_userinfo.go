package config

import (
	"math"
	"strconv"
	"strings"
)

func parseSubscriptionUserinfo(value string) (usedBytes, totalBytes *int64) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, nil
	}

	var (
		upload    uint64
		download  uint64
		total     uint64
		hasUpload bool
		hasDown   bool
		hasTotal  bool
	)

	normalized := strings.ReplaceAll(raw, ",", ";")
	for _, part := range strings.Split(normalized, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eq := strings.IndexByte(part, '=')
		if eq <= 0 || eq >= len(part)-1 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(part[:eq]))
		val := strings.TrimSpace(part[eq+1:])
		if key == "" || val == "" {
			continue
		}
		num, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "upload":
			upload = num
			hasUpload = true
		case "download":
			download = num
			hasDown = true
		case "total":
			total = num
			hasTotal = true
		}
	}

	if !hasUpload || !hasDown || !hasTotal {
		return nil, nil
	}

	if upload > math.MaxUint64-download {
		return nil, nil
	}
	used := upload + download
	if used > math.MaxInt64 || total > math.MaxInt64 {
		return nil, nil
	}

	used64 := int64(used)
	total64 := int64(total)
	return &used64, &total64
}
