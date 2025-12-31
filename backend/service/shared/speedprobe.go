package shared

import (
	"os"
	"strconv"
	"strings"
)

const (
	DefaultSpeedProbeMinSeconds            = 0.5
	DefaultSpeedProbeMinBytes        int64 = 5 * 1024 * 1024
	DefaultSpeedProbeFallbackSeconds       = 0.2
	DefaultSpeedProbeFallbackBytes         = 256 * 1024
)

// SpeedProbeConfig 提供测速阈值配置（可通过环境变量覆盖）。
// VEA_SPEEDTEST_MIN_SECONDS/VEA_SPEEDTEST_MIN_BYTES
// VEA_SPEEDTEST_FALLBACK_SECONDS/VEA_SPEEDTEST_FALLBACK_BYTES
type SpeedProbeConfig struct {
	MinSeconds      float64
	MinBytes        int64
	FallbackSeconds float64
	FallbackBytes   int64
}

var speedProbeConfig = loadSpeedProbeConfig()

func SpeedProbeConfigValue() SpeedProbeConfig {
	return speedProbeConfig
}

func loadSpeedProbeConfig() SpeedProbeConfig {
	cfg := SpeedProbeConfig{
		MinSeconds:      DefaultSpeedProbeMinSeconds,
		MinBytes:        DefaultSpeedProbeMinBytes,
		FallbackSeconds: DefaultSpeedProbeFallbackSeconds,
		FallbackBytes:   DefaultSpeedProbeFallbackBytes,
	}

	cfg.MinSeconds = parseFloatEnv("VEA_SPEEDTEST_MIN_SECONDS", cfg.MinSeconds)
	cfg.MinBytes = parseIntEnv("VEA_SPEEDTEST_MIN_BYTES", cfg.MinBytes)
	cfg.FallbackSeconds = parseFloatEnv("VEA_SPEEDTEST_FALLBACK_SECONDS", cfg.FallbackSeconds)
	cfg.FallbackBytes = parseIntEnv("VEA_SPEEDTEST_FALLBACK_BYTES", cfg.FallbackBytes)

	if cfg.MinSeconds <= 0 {
		cfg.MinSeconds = DefaultSpeedProbeMinSeconds
	}
	if cfg.MinBytes <= 0 {
		cfg.MinBytes = DefaultSpeedProbeMinBytes
	}
	if cfg.FallbackSeconds <= 0 {
		cfg.FallbackSeconds = cfg.MinSeconds
	}
	if cfg.FallbackBytes <= 0 {
		cfg.FallbackBytes = cfg.MinBytes
	}
	if cfg.FallbackSeconds > cfg.MinSeconds {
		cfg.FallbackSeconds = cfg.MinSeconds
	}
	if cfg.FallbackBytes > cfg.MinBytes {
		cfg.FallbackBytes = cfg.MinBytes
	}

	return cfg
}

func parseFloatEnv(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseIntEnv(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
