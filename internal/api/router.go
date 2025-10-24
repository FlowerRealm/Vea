package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"vea/internal/domain"
	"vea/internal/service"
)

type Router struct {
	service           *service.Service
	nodeNotFoundErr   error
	configNotFoundErr error
	geoNotFoundErr    error
	ruleNotFoundErr   error
}

func NewRouter(service *service.Service) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := &Router{service: service}
	r.nodeNotFoundErr, r.configNotFoundErr, r.geoNotFoundErr, r.ruleNotFoundErr = service.Errors()
	engine := gin.New()
	engine.Use(gin.Recovery(), gin.Logger())
	r.register(engine)
	return engine
}

func (r *Router) register(engine *gin.Engine) {
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now()})
	})

	engine.GET("/snapshot", func(c *gin.Context) {
		c.JSON(http.StatusOK, r.service.Snapshot())
	})

	nodes := engine.Group("/nodes")
	{
		nodes.GET("", r.listNodes)
		nodes.POST("", r.createNode)
		nodes.PUT(":id", r.updateNode)
		nodes.DELETE(":id", r.deleteNode)
		nodes.POST(":id/reset-traffic", r.resetNodeTraffic)
		nodes.POST(":id/traffic", r.incrementNodeTraffic)
		nodes.POST(":id/ping", r.pingNode)
		nodes.POST(":id/speedtest", r.speedtestNode)
	}

	configs := engine.Group("/configs")
	{
		configs.GET("", r.listConfigs)
		configs.POST("/import", r.importConfig)
		configs.PUT(":id", r.updateConfig)
		configs.DELETE(":id", r.deleteConfig)
		configs.POST(":id/refresh", r.refreshConfig)
		configs.POST(":id/traffic", r.incrementConfigTraffic)
	}

	geo := engine.Group("/geo")
	{
		geo.GET("", r.listGeo)
		geo.POST("", r.upsertGeo)
		geo.PUT(":id", r.upsertGeo)
		geo.DELETE(":id", r.deleteGeo)
		geo.POST(":id/refresh", r.refreshGeo)
	}

	traffic := engine.Group("/traffic")
	{
		traffic.GET("/profile", r.getTrafficProfile)
		traffic.PUT("/profile", r.updateTrafficProfile)
		traffic.GET("/rules", r.listTrafficRules)
		traffic.POST("/rules", r.createTrafficRule)
		traffic.PUT("/rules/:id", r.updateTrafficRule)
		traffic.DELETE("/rules/:id", r.deleteTrafficRule)
	}
}

type nodeRequest struct {
	Name     string              `json:"name" binding:"required"`
	Address  string              `json:"address" binding:"required"`
	Port     int                 `json:"port" binding:"required,min=1,max=65535"`
	Protocol domain.NodeProtocol `json:"protocol" binding:"required"`
	Tags     []string            `json:"tags"`
}

type nodeTrafficRequest struct {
	UploadBytes   int64 `json:"uploadBytes"`
	DownloadBytes int64 `json:"downloadBytes"`
}

func (r *Router) listNodes(c *gin.Context) {
	c.JSON(http.StatusOK, r.service.ListNodes())
}

func (r *Router) createNode(c *gin.Context) {
	var req nodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	node := domain.Node{
		Name:     req.Name,
		Address:  req.Address,
		Port:     req.Port,
		Protocol: req.Protocol,
		Tags:     req.Tags,
	}
	created := r.service.CreateNode(node)
	c.JSON(http.StatusCreated, created)
}

func (r *Router) updateNode(c *gin.Context) {
	var req nodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	id := c.Param("id")
	updated, err := r.service.UpdateNode(id, func(node domain.Node) (domain.Node, error) {
		node.Name = req.Name
		node.Address = req.Address
		node.Port = req.Port
		node.Protocol = req.Protocol
		node.Tags = req.Tags
		return node, nil
	})
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (r *Router) deleteNode(c *gin.Context) {
	id := c.Param("id")
	if err := r.service.DeleteNode(id); err != nil {
		r.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (r *Router) resetNodeTraffic(c *gin.Context) {
	id := c.Param("id")
	node, err := r.service.ResetNodeTraffic(id)
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, node)
}

func (r *Router) incrementNodeTraffic(c *gin.Context) {
	var req nodeTrafficRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	id := c.Param("id")
	node, err := r.service.IncrementNodeTraffic(id, req.UploadBytes, req.DownloadBytes)
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, node)
}

func (r *Router) pingNode(c *gin.Context) {
	id := c.Param("id")
	node, err := r.service.ProbeLatency(id)
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, node)
}

func (r *Router) speedtestNode(c *gin.Context) {
	id := c.Param("id")
	node, err := r.service.ProbeSpeed(id)
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, node)
}

type configRequest struct {
	Name               string              `json:"name" binding:"required"`
	Format             domain.ConfigFormat `json:"format" binding:"required"`
	Payload            string              `json:"payload" binding:"required"`
	AutoUpdateInterval int64               `json:"autoUpdateIntervalMinutes"`
	ExpireAt           *time.Time          `json:"expireAt"`
}

func (r *Router) listConfigs(c *gin.Context) {
	c.JSON(http.StatusOK, r.service.ListConfigs())
}

func (r *Router) importConfig(c *gin.Context) {
	var req configRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	now := time.Duration(req.AutoUpdateInterval) * time.Minute
	cfg := domain.Config{
		Name:               req.Name,
		Format:             req.Format,
		Payload:            req.Payload,
		AutoUpdateInterval: now,
		ExpireAt:           req.ExpireAt,
	}
	created := r.service.CreateConfig(cfg)
	c.JSON(http.StatusCreated, created)
}

func (r *Router) updateConfig(c *gin.Context) {
	var req configRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	id := c.Param("id")
	interval := time.Duration(req.AutoUpdateInterval) * time.Minute
	updated, err := r.service.UpdateConfig(id, func(cfg domain.Config) (domain.Config, error) {
		cfg.Name = req.Name
		cfg.Format = req.Format
		cfg.Payload = req.Payload
		cfg.AutoUpdateInterval = interval
		cfg.ExpireAt = req.ExpireAt
		return cfg, nil
	})
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (r *Router) deleteConfig(c *gin.Context) {
	id := c.Param("id")
	if err := r.service.DeleteConfig(id); err != nil {
		r.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (r *Router) refreshConfig(c *gin.Context) {
	id := c.Param("id")
	cfg, err := r.service.RefreshConfig(id)
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, cfg)
}

func (r *Router) incrementConfigTraffic(c *gin.Context) {
	var req nodeTrafficRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	id := c.Param("id")
	cfg, err := r.service.IncrementConfigTraffic(id, req.UploadBytes, req.DownloadBytes)
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, cfg)
}

type geoRequest struct {
	Name      string                 `json:"name" binding:"required"`
	Type      domain.GeoResourceType `json:"type" binding:"required"`
	SourceURL string                 `json:"sourceUrl" binding:"required"`
	Checksum  string                 `json:"checksum"`
	Version   string                 `json:"version"`
}

func (r *Router) listGeo(c *gin.Context) {
	c.JSON(http.StatusOK, r.service.ListGeo())
}

func (r *Router) upsertGeo(c *gin.Context) {
	var req geoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	id := c.Param("id")
	res := domain.GeoResource{
		ID:        id,
		Name:      req.Name,
		Type:      req.Type,
		SourceURL: req.SourceURL,
		Checksum:  req.Checksum,
		Version:   req.Version,
	}
	updated := r.service.UpsertGeo(res)
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	c.JSON(status, updated)
}

func (r *Router) deleteGeo(c *gin.Context) {
	id := c.Param("id")
	if err := r.service.DeleteGeo(id); err != nil {
		r.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (r *Router) refreshGeo(c *gin.Context) {
	id := c.Param("id")
	res, err := r.service.RefreshGeo(id)
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

type trafficProfileRequest struct {
	DefaultNodeID string     `json:"defaultNodeId"`
	DNS           dnsRequest `json:"dns"`
}

type dnsRequest struct {
	Strategy string   `json:"strategy"`
	Servers  []string `json:"servers"`
}

func (r *Router) getTrafficProfile(c *gin.Context) {
	c.JSON(http.StatusOK, r.service.GetTrafficProfile())
}

func (r *Router) updateTrafficProfile(c *gin.Context) {
	var req trafficProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	profile, err := r.service.UpdateTrafficProfile(func(profile domain.TrafficProfile) (domain.TrafficProfile, error) {
		profile.DefaultNodeID = req.DefaultNodeID
		profile.DNS = domain.DNSSetting{Strategy: req.DNS.Strategy, Servers: req.DNS.Servers}
		return profile, nil
	})
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

type trafficRuleRequest struct {
	Name     string   `json:"name" binding:"required"`
	Targets  []string `json:"targets" binding:"required"`
	NodeID   string   `json:"nodeId" binding:"required"`
	Priority int      `json:"priority"`
}

func (r *Router) listTrafficRules(c *gin.Context) {
	c.JSON(http.StatusOK, r.service.ListTrafficRules())
}

func (r *Router) createTrafficRule(c *gin.Context) {
	var req trafficRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	rule := domain.TrafficRule{
		Name:     req.Name,
		Targets:  req.Targets,
		NodeID:   req.NodeID,
		Priority: req.Priority,
	}
	created := r.service.CreateTrafficRule(rule)
	c.JSON(http.StatusCreated, created)
}

func (r *Router) updateTrafficRule(c *gin.Context) {
	var req trafficRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	id := c.Param("id")
	updated, err := r.service.UpdateTrafficRule(id, func(rule domain.TrafficRule) (domain.TrafficRule, error) {
		rule.Name = req.Name
		rule.Targets = req.Targets
		rule.NodeID = req.NodeID
		rule.Priority = req.Priority
		return rule, nil
	})
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (r *Router) deleteTrafficRule(c *gin.Context) {
	id := c.Param("id")
	if err := r.service.DeleteTrafficRule(id); err != nil {
		r.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func badRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func (r *Router) handleError(c *gin.Context, err error) {
	switch err {
	case r.nodeNotFoundErr, r.configNotFoundErr, r.geoNotFoundErr, r.ruleNotFoundErr:
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
