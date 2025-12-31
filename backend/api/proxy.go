package api

import (
	"errors"
	"io"
	"net/http"
	"runtime"
	"strconv"

	"github.com/gin-gonic/gin"

	"vea/backend/domain"
)

// Proxy handlers

func (r *Router) getProxyStatus(c *gin.Context) {
	status := r.service.GetProxyStatus()
	c.JSON(http.StatusOK, status)
}

func (r *Router) getProxyConfig(c *gin.Context) {
	config := r.service.GetProxyConfig()
	c.JSON(http.StatusOK, config)
}

func (r *Router) getKernelLogs(c *gin.Context) {
	var since int64
	if raw := c.Query("since"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			since = v
		}
	}
	snap := r.service.GetKernelLogs(since)
	c.JSON(http.StatusOK, snap)
}

func (r *Router) updateProxyConfig(c *gin.Context) {
	var req domain.ProxyConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}

	updated, err := r.service.UpdateProxyConfig(func(current domain.ProxyConfig) (domain.ProxyConfig, error) {
		if req.InboundMode != "" {
			current.InboundMode = req.InboundMode
		}
		if req.InboundPort != 0 {
			current.InboundPort = req.InboundPort
		}
		if req.InboundConfig != nil {
			current.InboundConfig = req.InboundConfig
		}
		if req.TUNSettings != nil {
			current.TUNSettings = req.TUNSettings
		}
		if req.ResolvedService != nil {
			current.ResolvedService = req.ResolvedService
		}
		if req.DNSConfig != nil {
			current.DNSConfig = req.DNSConfig
		}
		if req.LogConfig != nil {
			current.LogConfig = req.LogConfig
		}
		if req.PerformanceConfig != nil {
			current.PerformanceConfig = req.PerformanceConfig
		}
		if req.XrayConfig != nil {
			current.XrayConfig = req.XrayConfig
		}
		if req.PreferredEngine != "" {
			current.PreferredEngine = req.PreferredEngine
		}
		if req.FRouterID != "" {
			current.FRouterID = req.FRouterID
		}
		return current, nil
	})
	if err != nil {
		r.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (r *Router) startProxy(c *gin.Context) {
	// 允许空 body：表示按现有配置启动。
	var req domain.ProxyConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		if !errors.Is(err, io.EOF) {
			badRequest(c, err)
			return
		}
	}

	cfg := r.service.GetProxyConfig()
	if req.InboundMode != "" {
		cfg.InboundMode = req.InboundMode
	}
	if req.InboundPort != 0 {
		cfg.InboundPort = req.InboundPort
	}
	if req.InboundConfig != nil {
		cfg.InboundConfig = req.InboundConfig
	}
	if req.TUNSettings != nil {
		cfg.TUNSettings = req.TUNSettings
	}
	if req.ResolvedService != nil {
		cfg.ResolvedService = req.ResolvedService
	}
	if req.DNSConfig != nil {
		cfg.DNSConfig = req.DNSConfig
	}
	if req.LogConfig != nil {
		cfg.LogConfig = req.LogConfig
	}
	if req.PerformanceConfig != nil {
		cfg.PerformanceConfig = req.PerformanceConfig
	}
	if req.XrayConfig != nil {
		cfg.XrayConfig = req.XrayConfig
	}
	if req.PreferredEngine != "" {
		cfg.PreferredEngine = req.PreferredEngine
	}
	if req.FRouterID != "" {
		cfg.FRouterID = req.FRouterID
	}

	if err := r.service.StartProxy(cfg); err != nil {
		r.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "proxy started"})
}

func (r *Router) stopProxy(c *gin.Context) {
	if err := r.service.StopProxy(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "proxy stopped"})
}

// TUN capabilities check

func (r *Router) checkTUNCapabilities(c *gin.Context) {
	configured, err := r.service.CheckTUNCapabilities()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"configured": configured,
		"platform":   runtime.GOOS,
	}

	// 添加平台特定的设置命令
	switch runtime.GOOS {
	case "linux":
		response["setupCommand"] = "sudo ./vea setup-tun"
		response["description"] = "Creates vea-tun user and sets capabilities for sing-box (cap_net_admin,cap_net_bind_service,cap_net_raw)"
	case "windows":
		response["setupCommand"] = "Run Vea as Administrator"
		response["description"] = "TUN mode requires administrator privileges on Windows"
	case "darwin":
		response["setupCommand"] = "sudo vea"
		response["description"] = "TUN mode requires root privileges on macOS"
	default:
		response["setupCommand"] = "Not supported"
		response["description"] = "TUN mode is not supported on this platform"
	}

	c.JSON(http.StatusOK, response)
}

func (r *Router) setupTUN(c *gin.Context) {
	err := r.service.SetupTUN()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "TUN setup successful"})
}
