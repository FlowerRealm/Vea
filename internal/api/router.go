package api

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vea/internal/domain"
	"vea/internal/service"
)

type Router struct {
	webFS fs.FS
	service              *service.Service
	nodeNotFoundErr      error
	configNotFoundErr    error
	geoNotFoundErr       error
	ruleNotFoundErr      error
	componentNotFoundErr error
}

func NewRouter(service *service.Service, webFS fs.FS) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := &Router{service: service, webFS: webFS}
	r.nodeNotFoundErr, r.configNotFoundErr, r.geoNotFoundErr, r.ruleNotFoundErr, r.componentNotFoundErr = service.Errors()
	engine := gin.New()
	engine.Use(gin.Recovery())
	r.register(engine)
	return engine
}

func (r *Router) register(engine *gin.Engine) {
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now()})
	})

	engine.GET("/", func(c *gin.Context) {
		data, err := fs.ReadFile(r.webFS, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "404 not found: %v", err)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
	engine.StaticFS("/ui", http.FS(r.webFS))

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
		nodes.POST(":id/select", r.selectNode)
		nodes.POST("/bulk/ping", r.bulkPingNodes)
		nodes.POST("/reset-speed", r.resetNodeSpeed)
	}

	configs := engine.Group("/configs")
	{
		configs.GET("", r.listConfigs)
		configs.POST("/import", r.importConfig)
		configs.PUT(":id", r.updateConfig)
		configs.DELETE(":id", r.deleteConfig)
		configs.POST(":id/refresh", r.refreshConfig)
		configs.POST(":id/pull-nodes", r.pullConfigNodes)
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

	components := engine.Group("/components")
	{
		components.GET("", r.listComponents)
		components.POST("", r.createComponent)
		components.PUT(":id", r.updateComponent)
		components.DELETE(":id", r.deleteComponent)
		components.POST(":id/install", r.installComponent)
	}

	settings := engine.Group("/settings")
	{
		settings.GET("/system-proxy", r.getSystemProxySettings)
		settings.PUT("/system-proxy", r.updateSystemProxySettings)
	}

	xray := engine.Group("/xray")
	{
		xray.GET("/status", r.xrayStatus)
		xray.POST("/start", r.startXray)
		xray.POST("/stop", r.stopXray)
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

func (r *Router) xrayStatus(c *gin.Context) {
	c.JSON(http.StatusOK, r.service.XrayStatus())
}

func (r *Router) startXray(c *gin.Context) {
	var req struct {
		ActiveNodeID string `json:"activeNodeId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		badRequest(c, err)
		return
	}
	if err := r.service.EnableXray(req.ActiveNodeID); err != nil {
		r.handleError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (r *Router) stopXray(c *gin.Context) {
	if err := r.service.DisableXray(); err != nil {
		r.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type nodeCreateRequest struct {
	ShareLink string              `json:"shareLink"`
	Name      string              `json:"name"`
	Address   string              `json:"address"`
	Port      int                 `json:"port"`
	Protocol  domain.NodeProtocol `json:"protocol"`
	Tags      []string            `json:"tags"`
}

type nodeUpdateRequest struct {
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
	nodes := r.service.ListNodes()
	c.JSON(http.StatusOK, gin.H{
		"nodes":              nodes,
		"activeNodeId":       r.service.ActiveXrayNodeID(),
		"lastSelectedNodeId": r.service.LastSelectedNodeID(),
	})
}

func (r *Router) createNode(c *gin.Context) {
	var req nodeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	if share := strings.TrimSpace(req.ShareLink); share != "" {
		created, err := r.service.CreateNodeFromShare(share)
		if err != nil {
			badRequest(c, err)
			return
		}
		c.JSON(http.StatusCreated, created)
		return
	}
	if req.Name == "" || req.Address == "" || req.Port <= 0 || req.Protocol == "" {
		badRequest(c, errors.New("name, address, port and protocol are required"))
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
	var req nodeUpdateRequest
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
	r.service.PingAsync(id)
	c.Status(http.StatusAccepted)
}

func (r *Router) speedtestNode(c *gin.Context) {
	id := c.Param("id")
	r.service.SpeedtestAsync(id)
	c.Status(http.StatusAccepted)
}

func (r *Router) selectNode(c *gin.Context) {
	id := c.Param("id")
	// 非阻塞切换，立即返回，降低前端等待
	r.service.SwitchXrayNodeAsync(id)
	c.Status(http.StatusAccepted)
}

func (r *Router) bulkPingNodes(c *gin.Context) {
	ids := struct {
		IDs []string `json:"ids"`
	}{}
	if err := c.ShouldBindJSON(&ids); err != nil {
		badRequest(c, err)
		return
	}
	var targetIDs []string
	if len(ids.IDs) == 0 {
		for _, node := range r.service.ListNodes() {
			targetIDs = append(targetIDs, node.ID)
		}
	} else {
		targetIDs = ids.IDs
	}
	for _, id := range targetIDs {
		if id == "" {
			continue
		}
		r.service.PingAsync(id)
	}
	c.Status(http.StatusAccepted)
}

func (r *Router) resetNodeSpeed(c *gin.Context) {
	ids := struct {
		IDs []string `json:"ids"`
	}{}
	if err := c.ShouldBindJSON(&ids); err != nil && !errors.Is(err, io.EOF) {
		badRequest(c, err)
		return
	}
	r.service.ResetNodeSpeeds(ids.IDs)
	c.Status(http.StatusNoContent)
}

type configRequest struct {
	Name               string              `json:"name" binding:"required"`
	Format             domain.ConfigFormat `json:"format" binding:"required"`
	Payload            string              `json:"payload"`
	SourceURL          string              `json:"sourceUrl"`
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
	if strings.TrimSpace(req.SourceURL) == "" {
		badRequest(c, errors.New("sourceUrl is required"))
		return
	}
	now := time.Duration(req.AutoUpdateInterval) * time.Minute
	cfg := domain.Config{
		Name:               req.Name,
		Format:             req.Format,
		Payload:            req.Payload,
		SourceURL:          req.SourceURL,
		AutoUpdateInterval: now,
		ExpireAt:           req.ExpireAt,
	}
	created, err := r.service.CreateConfig(cfg)
	if err != nil {
		badRequest(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (r *Router) updateConfig(c *gin.Context) {
	var req configRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	if strings.TrimSpace(req.SourceURL) == "" {
		badRequest(c, errors.New("sourceUrl is required"))
		return
	}
	id := c.Param("id")
	interval := time.Duration(req.AutoUpdateInterval) * time.Minute
	updated, err := r.service.UpdateConfig(id, func(cfg domain.Config) (domain.Config, error) {
		cfg.Name = req.Name
		cfg.Format = req.Format
		cfg.Payload = req.Payload
		cfg.SourceURL = req.SourceURL
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

func (r *Router) pullConfigNodes(c *gin.Context) {
	id := c.Param("id")
	nodes, err := r.service.SyncConfigNodes(id)
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, nodes)
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

type componentRequest struct {
	Name                  string                   `json:"name"`
	Kind                  domain.CoreComponentKind `json:"kind"`
	SourceURL             string                   `json:"sourceUrl"`
	ArchiveType           string                   `json:"archiveType"`
	AutoUpdateIntervalMin int64                    `json:"autoUpdateIntervalMinutes"`
}

func (r *Router) listComponents(c *gin.Context) {
	c.JSON(http.StatusOK, r.service.ListComponents())
}

func (r *Router) createComponent(c *gin.Context) {
	var req componentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	interval := time.Duration(0)
	if req.AutoUpdateIntervalMin > 0 {
		interval = time.Duration(req.AutoUpdateIntervalMin) * time.Minute
	}
	component := domain.CoreComponent{
		Name:               req.Name,
		Kind:               req.Kind,
		SourceURL:          req.SourceURL,
		ArchiveType:        req.ArchiveType,
		AutoUpdateInterval: interval,
	}
	created, err := r.service.CreateComponent(component)
	if err != nil {
		badRequest(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (r *Router) updateComponent(c *gin.Context) {
	var req componentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	interval := time.Duration(0)
	if req.AutoUpdateIntervalMin > 0 {
		interval = time.Duration(req.AutoUpdateIntervalMin) * time.Minute
	}
	id := c.Param("id")
	updated, err := r.service.UpdateComponent(id, func(component domain.CoreComponent) (domain.CoreComponent, error) {
		component.Name = req.Name
		if req.Kind != "" {
			component.Kind = req.Kind
		}
		component.SourceURL = req.SourceURL
		if req.ArchiveType != "" {
			component.ArchiveType = req.ArchiveType
		}
		component.AutoUpdateInterval = interval
		return component, nil
	})
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (r *Router) deleteComponent(c *gin.Context) {
	id := c.Param("id")
	if err := r.service.DeleteComponent(id); err != nil {
		r.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (r *Router) installComponent(c *gin.Context) {
	id := c.Param("id")
	component, err := r.service.InstallComponent(id)
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, component)
}

func (r *Router) getSystemProxySettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"settings": r.service.SystemProxySettings(),
		"message":  "",
	})
}

type systemProxyRequest struct {
	Enabled     bool     `json:"enabled"`
	IgnoreHosts []string `json:"ignoreHosts"`
}

func (r *Router) updateSystemProxySettings(c *gin.Context) {
	var req systemProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	updated, err := r.service.UpdateSystemProxySettings(domain.SystemProxySettings{
		Enabled:     req.Enabled,
		IgnoreHosts: req.IgnoreHosts,
	})
	if err != nil {
		if errors.Is(err, service.ErrProxyUnsupported) || errors.Is(err, service.ErrProxyXrayNotRunning) {
			c.JSON(http.StatusOK, gin.H{
				"settings": updated,
				"message":  err.Error(),
			})
			return
		}
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"settings": updated,
		"message":  "",
	})
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
	case r.nodeNotFoundErr, r.configNotFoundErr, r.geoNotFoundErr, r.ruleNotFoundErr, r.componentNotFoundErr:
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		if errors.Is(err, service.ErrXrayNotInstalled) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
