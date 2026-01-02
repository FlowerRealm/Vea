package api

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/service"
	"vea/backend/service/nodegroup"
)

type Router struct {
	service              *service.Facade
	nodeNotFoundErr      error
	frouterNotFoundErr   error
	configNotFoundErr    error
	geoNotFoundErr       error
	componentNotFoundErr error
}

func NewRouter(svc *service.Facade) *gin.Engine {
	r := &Router{service: svc}
	r.nodeNotFoundErr, r.frouterNotFoundErr, r.configNotFoundErr, r.geoNotFoundErr, r.componentNotFoundErr = svc.Errors()
	engine := gin.New()
	engine.Use(gin.Recovery())
	r.register(engine)
	return engine
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func (r *Router) register(engine *gin.Engine) {
	engine.Use(corsMiddleware())

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
		nodes.POST(":id/ping", r.pingNode)
		nodes.POST(":id/speedtest", r.speedtestNode)
		nodes.POST("/bulk/ping", r.bulkPingNodes)
		nodes.POST("/bulk/speedtest", r.bulkSpeedtestNodes)
	}

	frouters := engine.Group("/frouters")
	{
		frouters.GET("", r.listFRouters)
		frouters.POST("", r.createFRouter)
		frouters.PUT(":id", r.updateFRouter)
		frouters.DELETE(":id", r.deleteFRouter)
		frouters.POST(":id/ping", r.pingFRouter)
		frouters.POST(":id/speedtest", r.speedtestFRouter)
		frouters.POST("/bulk/ping", r.bulkPingFRouters)
		frouters.POST("/reset-speed", r.resetFRouterSpeed)

		// FRouter 图编辑
		frouters.GET(":id/graph", r.getFRouterGraph)
		frouters.PUT(":id/graph", r.saveFRouterGraph)
		frouters.POST(":id/graph/validate", r.validateFRouterGraph)
	}

	configs := engine.Group("/configs")
	{
		configs.GET("", r.listConfigs)
		configs.POST("/import", r.importConfig)
		configs.PUT(":id", r.updateConfig)
		configs.DELETE(":id", r.deleteConfig)
		configs.POST(":id/refresh", r.refreshConfig)
		configs.POST(":id/pull-nodes", r.pullConfigNodes)
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
		settings.GET("/frontend", r.getFrontendSettings)
		settings.PUT("/frontend", r.saveFrontendSettings)
	}

	proxy := engine.Group("/proxy")
	{
		proxy.GET("/status", r.getProxyStatus)
		proxy.GET("/kernel/logs", r.getKernelLogs)
		proxy.GET("/config", r.getProxyConfig)
		proxy.PUT("/config", r.updateProxyConfig)
		proxy.POST("/start", r.startProxy)
		proxy.POST("/stop", r.stopProxy)
	}

	// TUN API
	engine.GET("/tun/check", r.checkTUNCapabilities)
	engine.POST("/tun/setup", r.setupTUN)

	// Engine 推荐 API
	eng := engine.Group("/engine")
	{
		eng.GET("/recommend", r.getEngineRecommendation)
		eng.GET("/status", r.getEngineStatus)
	}

	// IP Geo API
	engine.GET("/ip/geo", r.getIPGeo)

	// 图编辑属于 FRouter：/frouters/:id/graph
}

type frouterCreateRequest struct {
	Name       string                     `json:"name" binding:"required"`
	ChainProxy *domain.ChainProxySettings `json:"chainProxy,omitempty"`
	Tags       []string                   `json:"tags,omitempty"`
}

type frouterUpdateRequest struct {
	Name       string                     `json:"name" binding:"required"`
	ChainProxy *domain.ChainProxySettings `json:"chainProxy,omitempty"`
	Tags       []string                   `json:"tags,omitempty"`
}

type nodeRequest struct {
	Name      string                `json:"name" binding:"required"`
	Address   string                `json:"address" binding:"required"`
	Port      int                   `json:"port" binding:"required"`
	Protocol  domain.NodeProtocol   `json:"protocol" binding:"required"`
	Tags      []string              `json:"tags,omitempty"`
	Security  *domain.NodeSecurity  `json:"security,omitempty"`
	Transport *domain.NodeTransport `json:"transport,omitempty"`
	TLS       *domain.NodeTLS       `json:"tls,omitempty"`
}

func (r *Router) listFRouters(c *gin.Context) {
	frouters := r.service.ListFRouters()
	c.JSON(http.StatusOK, gin.H{
		"frouters": frouters,
	})
}

func (r *Router) listNodes(c *gin.Context) {
	nodes := r.service.ListNodes()
	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
	})
}

func (r *Router) createNode(c *gin.Context) {
	var req nodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	node, err := buildNodeFromRequest(req)
	if err != nil {
		r.handleError(c, err)
		return
	}
	created, err := r.service.CreateNode(node)
	if err != nil {
		r.handleError(c, err)
		return
	}
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
		if strings.TrimSpace(node.SourceConfigID) != "" {
			return domain.Node{}, fmt.Errorf("%w: subscription node is read-only", repository.ErrInvalidData)
		}
		next, err := buildNodeFromRequest(req)
		if err != nil {
			return domain.Node{}, err
		}
		next.SourceConfigID = node.SourceConfigID
		return next, nil
	})
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func buildNodeFromRequest(req nodeRequest) (domain.Node, error) {
	if err := validateNodeRequest(req); err != nil {
		return domain.Node{}, err
	}
	node := domain.Node{
		Name:      strings.TrimSpace(req.Name),
		Address:   strings.TrimSpace(req.Address),
		Port:      req.Port,
		Protocol:  req.Protocol,
		Tags:      req.Tags,
		Security:  req.Security,
		Transport: req.Transport,
		TLS:       req.TLS,
	}
	if node.Protocol == domain.ProtocolVMess && node.Security != nil {
		if node.Security.Encryption == "" && node.Security.Method != "" {
			node.Security.Encryption = node.Security.Method
		}
		if node.Security.Method == "" && node.Security.Encryption != "" {
			node.Security.Method = node.Security.Encryption
		}
	}
	if node.TLS != nil && node.TLS.Enabled && node.TLS.Type == "" {
		node.TLS.Type = "tls"
	}
	return node, nil
}

func validateNodeRequest(req nodeRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("%w: name is required", repository.ErrInvalidData)
	}
	if strings.TrimSpace(req.Address) == "" {
		return fmt.Errorf("%w: address is required", repository.ErrInvalidData)
	}
	if req.Port <= 0 || req.Port > 65535 {
		return fmt.Errorf("%w: port is invalid", repository.ErrInvalidData)
	}
	switch req.Protocol {
	case domain.ProtocolVLESS, domain.ProtocolVMess, domain.ProtocolTrojan, domain.ProtocolShadowsocks, domain.ProtocolHysteria2, domain.ProtocolTUIC:
	default:
		return fmt.Errorf("%w: protocol is invalid", repository.ErrInvalidData)
	}
	switch req.Protocol {
	case domain.ProtocolShadowsocks:
		if req.Security == nil || strings.TrimSpace(req.Security.Method) == "" || strings.TrimSpace(req.Security.Password) == "" {
			return fmt.Errorf("%w: shadowsocks requires method and password", repository.ErrInvalidData)
		}
	case domain.ProtocolVMess, domain.ProtocolVLESS:
		if req.Security == nil || strings.TrimSpace(req.Security.UUID) == "" {
			return fmt.Errorf("%w: vmess/vless requires uuid", repository.ErrInvalidData)
		}
	case domain.ProtocolTrojan:
		if req.Security == nil || strings.TrimSpace(req.Security.Password) == "" {
			return fmt.Errorf("%w: trojan requires password", repository.ErrInvalidData)
		}
	case domain.ProtocolHysteria2:
		if req.Security == nil || strings.TrimSpace(req.Security.Password) == "" {
			return fmt.Errorf("%w: hysteria2 requires password", repository.ErrInvalidData)
		}
	case domain.ProtocolTUIC:
		if req.Security == nil || strings.TrimSpace(req.Security.UUID) == "" || strings.TrimSpace(req.Security.Password) == "" {
			return fmt.Errorf("%w: tuic requires uuid and password", repository.ErrInvalidData)
		}
	}
	return nil
}

func (r *Router) pingNode(c *gin.Context) {
	id := c.Param("id")
	r.service.MeasureNodeLatencyAsync(id)
	c.Status(http.StatusAccepted)
}

func (r *Router) speedtestNode(c *gin.Context) {
	id := c.Param("id")
	r.service.MeasureNodeSpeedAsync(id)
	c.Status(http.StatusAccepted)
}

func (r *Router) bulkPingNodes(c *gin.Context) {
	ids := struct {
		IDs []string `json:"ids"`
	}{}
	if err := c.ShouldBindJSON(&ids); err != nil && !errors.Is(err, io.EOF) {
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
		if strings.TrimSpace(id) == "" {
			continue
		}
		r.service.MeasureNodeLatencyAsync(id)
	}
	c.Status(http.StatusAccepted)
}

func (r *Router) bulkSpeedtestNodes(c *gin.Context) {
	ids := struct {
		IDs []string `json:"ids"`
	}{}
	if err := c.ShouldBindJSON(&ids); err != nil && !errors.Is(err, io.EOF) {
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
		if strings.TrimSpace(id) == "" {
			continue
		}
		r.service.MeasureNodeSpeedAsync(id)
	}
	c.Status(http.StatusAccepted)
}

func (r *Router) createFRouter(c *gin.Context) {
	var req frouterCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	frouter := domain.FRouter{
		Name: req.Name,
		Tags: req.Tags,
	}
	if req.ChainProxy != nil {
		frouter.ChainProxy = *req.ChainProxy
	}
	if len(frouter.ChainProxy.Slots) == 0 {
		frouter.ChainProxy.Slots = []domain.SlotNode{
			{ID: "slot-1", Name: "配置槽"},
		}
	}
	if len(frouter.ChainProxy.Edges) == 0 {
		frouter.ChainProxy.Edges = []domain.ProxyEdge{
			{
				ID:       uuid.NewString(),
				From:     domain.EdgeNodeLocal,
				To:       domain.EdgeNodeDirect,
				Priority: 0,
				Enabled:  true,
			},
		}
	}
	frouter.ChainProxy.Edges = normalizeChainEdges(frouter.ChainProxy.Edges)
	if _, err := nodegroup.CompileFRouter(frouter, r.service.ListNodes()); err != nil {
		var ce *nodegroup.CompileError
		if errors.As(err, &ce) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":    "invalid frouter",
				"problems": ce.Problems,
			})
			return
		}
		badRequest(c, err)
		return
	}
	created := r.service.CreateFRouter(frouter)
	c.JSON(http.StatusCreated, created)
}

func (r *Router) updateFRouter(c *gin.Context) {
	var req frouterUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}
	id := c.Param("id")
	updated, err := r.service.UpdateFRouter(id, func(frouter domain.FRouter) (domain.FRouter, error) {
		frouter.Name = req.Name
		if req.ChainProxy != nil {
			frouter.ChainProxy = *req.ChainProxy
		}
		frouter.ChainProxy.Edges = normalizeChainEdges(frouter.ChainProxy.Edges)
		if _, err := nodegroup.CompileFRouter(frouter, r.service.ListNodes()); err != nil {
			return domain.FRouter{}, err
		}
		frouter.Tags = req.Tags
		return frouter, nil
	})
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (r *Router) deleteFRouter(c *gin.Context) {
	id := c.Param("id")
	if err := r.service.DeleteFRouter(id); err != nil {
		r.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (r *Router) pingFRouter(c *gin.Context) {
	id := c.Param("id")
	r.service.MeasureFRouterLatencyAsync(id)
	c.Status(http.StatusAccepted)
}

func (r *Router) speedtestFRouter(c *gin.Context) {
	id := c.Param("id")
	r.service.MeasureFRouterSpeedAsync(id)
	c.Status(http.StatusAccepted)
}

func (r *Router) bulkPingFRouters(c *gin.Context) {
	ids := struct {
		IDs []string `json:"ids"`
	}{}
	if err := c.ShouldBindJSON(&ids); err != nil {
		badRequest(c, err)
		return
	}
	var targetIDs []string
	if len(ids.IDs) == 0 {
		for _, frouter := range r.service.ListFRouters() {
			targetIDs = append(targetIDs, frouter.ID)
		}
	} else {
		targetIDs = ids.IDs
	}
	for _, id := range targetIDs {
		if id == "" {
			continue
		}
		r.service.MeasureFRouterLatencyAsync(id)
	}
	c.Status(http.StatusAccepted)
}

func (r *Router) resetFRouterSpeed(c *gin.Context) {
	ids := struct {
		IDs []string `json:"ids"`
	}{}
	if err := c.ShouldBindJSON(&ids); err != nil && !errors.Is(err, io.EOF) {
		badRequest(c, err)
		return
	}
	targetIDs := make(map[string]struct{})
	if len(ids.IDs) == 0 {
		for _, frouter := range r.service.ListFRouters() {
			targetIDs[frouter.ID] = struct{}{}
		}
	} else {
		for _, id := range ids.IDs {
			targetIDs[id] = struct{}{}
		}
	}
	for _, frouter := range r.service.ListFRouters() {
		if _, ok := targetIDs[frouter.ID]; !ok {
			continue
		}
		_, _ = r.service.UpdateFRouter(frouter.ID, func(rp domain.FRouter) (domain.FRouter, error) {
			rp.LastSpeedMbps = 0
			rp.LastSpeedAt = time.Time{}
			rp.LastSpeedError = ""
			return rp, nil
		})
	}
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
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
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
	Name        string                   `json:"name"`
	Kind        domain.CoreComponentKind `json:"kind"`
	SourceURL   string                   `json:"sourceUrl"`
	ArchiveType string                   `json:"archiveType"`
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
	component := domain.CoreComponent{
		Name:        req.Name,
		Kind:        req.Kind,
		SourceURL:   req.SourceURL,
		ArchiveType: req.ArchiveType,
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
	component, err := r.service.InstallComponentAsync(id)
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, component)
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
	updated, message, err := r.service.UpdateSystemProxySettings(domain.SystemProxySettings{
		Enabled:     req.Enabled,
		IgnoreHosts: req.IgnoreHosts,
	})
	if err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"settings": updated,
		"message":  message,
	})
}

func (r *Router) getFrontendSettings(c *gin.Context) {
	settings := r.service.GetFrontendSettings()
	c.JSON(http.StatusOK, settings)
}

func (r *Router) saveFrontendSettings(c *gin.Context) {
	var settings map[string]interface{}
	if err := c.ShouldBindJSON(&settings); err != nil {
		badRequest(c, err)
		return
	}
	if err := r.service.SaveFrontendSettings(settings); err != nil {
		r.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (r *Router) getIPGeo(c *gin.Context) {
	result, err := r.service.GetIPGeo()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ip":       "",
			"location": "",
			"asn":      "",
			"isp":      "",
			"error":    err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

func badRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func (r *Router) handleError(c *gin.Context, err error) {
	var ce *nodegroup.CompileError
	if errors.As(err, &ce) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "invalid frouter",
			"problems": ce.Problems,
		})
		return
	}

	if errors.Is(err, repository.ErrInvalidID) || errors.Is(err, repository.ErrInvalidData) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if errors.Is(err, r.frouterNotFoundErr) ||
		errors.Is(err, r.nodeNotFoundErr) ||
		errors.Is(err, r.configNotFoundErr) ||
		errors.Is(err, r.geoNotFoundErr) ||
		errors.Is(err, r.componentNotFoundErr) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

// ==================== FRouter Graph API ====================

// frouterGraphRequest 图保存请求
type frouterGraphRequest struct {
	Edges     []domain.ProxyEdge              `json:"edges"`
	Positions map[string]domain.GraphPosition `json:"positions"`
	Slots     []domain.SlotNode               `json:"slots"`
}

// frouterGraphResponse 图读取响应
type frouterGraphResponse struct {
	Edges     []domain.ProxyEdge              `json:"edges"`
	Positions map[string]domain.GraphPosition `json:"positions"`
	Slots     []domain.SlotNode               `json:"slots"`
	UpdatedAt time.Time                       `json:"updatedAt"`
}

// validateGraphResponse 图验证响应
type validateGraphResponse struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

func (r *Router) resolveFRouterForGraph(c *gin.Context) (domain.FRouter, bool) {
	frouterID := strings.TrimSpace(c.Param("id"))
	if frouterID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "frouter not found"})
		return domain.FRouter{}, false
	}
	frouter, err := r.service.GetFRouter(frouterID)
	if err != nil {
		r.handleError(c, err)
		return domain.FRouter{}, false
	}
	return frouter, true
}

// getFRouterGraph 获取完整图数据
func (r *Router) getFRouterGraph(c *gin.Context) {
	frouter, ok := r.resolveFRouterForGraph(c)
	if !ok {
		return
	}
	settings := frouter.ChainProxy

	c.JSON(http.StatusOK, frouterGraphResponse{
		Edges:     settings.Edges,
		Positions: settings.Positions,
		Slots:     settings.Slots,
		UpdatedAt: settings.UpdatedAt,
	})
}

// saveFRouterGraph 保存完整图数据
func (r *Router) saveFRouterGraph(c *gin.Context) {
	var req frouterGraphRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}

	frouter, ok := r.resolveFRouterForGraph(c)
	if !ok {
		return
	}

	// 保存前先做语义校验 + 归一化（失败即失败）
	draft := frouter
	if req.Edges != nil {
		draft.ChainProxy.Edges = normalizeChainEdges(req.Edges)
	} else {
		draft.ChainProxy.Edges = []domain.ProxyEdge{}
	}
	if req.Positions != nil {
		draft.ChainProxy.Positions = req.Positions
	} else {
		draft.ChainProxy.Positions = make(map[string]domain.GraphPosition)
	}
	if req.Slots != nil {
		draft.ChainProxy.Slots = req.Slots
	} else {
		draft.ChainProxy.Slots = []domain.SlotNode{}
	}

	if _, err := nodegroup.CompileFRouter(draft, r.service.ListNodes()); err != nil {
		var ce *nodegroup.CompileError
		if errors.As(err, &ce) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":    "invalid frouter graph",
				"problems": ce.Problems,
			})
			return
		}
		badRequest(c, err)
		return
	}

	updated, err := r.service.UpdateFRouter(frouter.ID, func(frouter domain.FRouter) (domain.FRouter, error) {
		frouter.ChainProxy.Edges = draft.ChainProxy.Edges
		frouter.ChainProxy.Positions = draft.ChainProxy.Positions
		frouter.ChainProxy.Slots = draft.ChainProxy.Slots
		frouter.ChainProxy.UpdatedAt = time.Now()
		return frouter, nil
	})
	if err != nil {
		r.handleError(c, err)
		return
	}

	// 如果代理正在运行，自动重启以应用新配置
	status := r.service.GetProxyStatus()
	if running, ok := status["running"].(bool); ok && running {
		cfg := r.service.GetProxyConfig()
		if cfg.FRouterID == "" {
			if id, ok := status["frouterId"].(string); ok && id != "" {
				cfg.FRouterID = id
			}
		}
		if cfg.FRouterID != "" {
			c.Header("X-Vea-Effects", "proxy_restart_scheduled")
			log.Printf("[FRouterGraph] 图配置已更新，重启代理以应用更改")
			go func(cfg domain.ProxyConfig) {
				// StopProxy 会强制关闭系统代理并持久化（防止“内核停了但系统代理还指向黑洞”）。
				// 但这里是“重启内核以应用配置”，用户期望系统代理在重启后保持原状态，因此需要恢复。
				originalSystemProxy := r.service.SystemProxySettings()
				restoreSystemProxy := originalSystemProxy.Enabled

				if err := r.service.StopProxy(); err != nil {
					log.Printf("[FRouterGraph] 停止代理失败: %v", err)
					return
				}
				if err := r.service.StartProxy(cfg); err != nil {
					log.Printf("[FRouterGraph] 重启代理失败: %v", err)
					return
				}

				if restoreSystemProxy {
					originalSystemProxy.Enabled = true
					if _, _, err := r.service.UpdateSystemProxySettings(originalSystemProxy); err != nil {
						log.Printf("[FRouterGraph] 恢复系统代理失败: %v", err)
					}
				}
			}(cfg)
		}
	}

	c.JSON(http.StatusOK, updated)
}

func normalizeChainEdges(edges []domain.ProxyEdge) []domain.ProxyEdge {
	if len(edges) == 0 {
		return []domain.ProxyEdge{}
	}
	out := make([]domain.ProxyEdge, len(edges))
	copy(out, edges)
	for i := range out {
		if !out[i].Enabled {
			continue
		}
		isDefault, err := nodegroup.IsDefaultSelectionEdge(out[i])
		if err != nil {
			continue
		}
		if isDefault {
			out[i].Priority = 0
		}
	}
	return out
}

// validateFRouterGraph 验证图配置
func (r *Router) validateFRouterGraph(c *gin.Context) {
	var req frouterGraphRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err)
		return
	}

	frouter, ok := r.resolveFRouterForGraph(c)
	if !ok {
		return
	}

	// 仅验证语义，不持久化
	draft := frouter
	if req.Edges != nil {
		draft.ChainProxy.Edges = req.Edges
	} else {
		draft.ChainProxy.Edges = []domain.ProxyEdge{}
	}
	if req.Slots != nil {
		draft.ChainProxy.Slots = req.Slots
	} else {
		draft.ChainProxy.Slots = []domain.SlotNode{}
	}
	if req.Positions != nil {
		draft.ChainProxy.Positions = req.Positions
	}

	compiled, err := nodegroup.CompileFRouter(draft, r.service.ListNodes())
	if err != nil {
		var ce *nodegroup.CompileError
		if errors.As(err, &ce) {
			c.JSON(http.StatusOK, validateGraphResponse{
				Valid:    false,
				Errors:   ce.Problems,
				Warnings: []string{},
			})
			return
		}
		c.JSON(http.StatusOK, validateGraphResponse{
			Valid:    false,
			Errors:   []string{err.Error()},
			Warnings: []string{},
		})
		return
	}

	c.JSON(http.StatusOK, validateGraphResponse{
		Valid:    true,
		Errors:   []string{},
		Warnings: compiled.Warnings,
	})
}

// ========== Engine 推荐 API ==========

// getEngineRecommendation 获取引擎推荐
func (r *Router) getEngineRecommendation(c *gin.Context) {
	recommendation := r.service.RecommendEngine()
	c.JSON(http.StatusOK, recommendation)
}

// getEngineStatus 获取引擎状态
func (r *Router) getEngineStatus(c *gin.Context) {
	status := r.service.GetEngineStatus()
	c.JSON(http.StatusOK, status)
}
