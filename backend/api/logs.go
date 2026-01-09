package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (r *Router) getAppLogs(c *gin.Context) {
	var since int64
	if raw := c.Query("since"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			badRequest(c, errors.New("invalid 'since' parameter: must be a non-negative integer"))
			return
		}
		since = v
	}
	snap := r.service.GetAppLogs(since)
	c.JSON(http.StatusOK, snap)
}
