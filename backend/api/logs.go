package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (r *Router) getAppLogs(c *gin.Context) {
	var since int64
	if raw := c.Query("since"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			since = v
		}
	}
	snap := r.service.GetAppLogs(since)
	c.JSON(http.StatusOK, snap)
}
