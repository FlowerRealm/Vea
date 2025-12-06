package api

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"

	"vea/backend/domain"
)

// ProxyProfile handlers

func (r *Router) listProxyProfiles(c *gin.Context) {
	profiles := r.service.ListProxyProfiles()
	c.JSON(http.StatusOK, profiles)
}

func (r *Router) createProxyProfile(c *gin.Context) {
	var req domain.ProxyProfile
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile, err := r.service.CreateProxyProfile(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, profile)
}

func (r *Router) getProxyProfile(c *gin.Context) {
	id := c.Param("id")
	profile, err := r.service.GetProxyProfile(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

func (r *Router) updateProxyProfile(c *gin.Context) {
	id := c.Param("id")
	var req domain.ProxyProfile
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile, err := r.service.UpdateProxyProfile(id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, profile)
}

func (r *Router) deleteProxyProfile(c *gin.Context) {
	id := c.Param("id")
	if err := r.service.DeleteProxyProfile(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "profile deleted"})
}

func (r *Router) startProxyProfile(c *gin.Context) {
	id := c.Param("id")
	if err := r.service.StartProxyWithProfile(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "proxy started"})
}

// Proxy control handlers

func (r *Router) getProxyStatus(c *gin.Context) {
	status := r.service.GetProxyStatus()
	c.JSON(http.StatusOK, status)
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
		response["setupCommand"] = "sudo vea setup-tun"
		response["description"] = "Creates vea-tun user with CAP_NET_ADMIN capability"
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
