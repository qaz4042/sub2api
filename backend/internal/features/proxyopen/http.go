package proxyopen

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Register must only be called on the admin-authenticated proxy route group.
func (r *Runner) Register(group *gin.RouterGroup) {
	group.GET("/openai-monitor/overview", r.overview)
	group.GET("/openai-monitor/trends", r.trends)
	group.GET("/:id/openai-monitor", r.handle)
	group.GET("/:id/openai-monitor/config", r.readConfig)
	group.PUT("/:id/openai-monitor", r.handle)
}
func (r *Runner) handle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if c.Request.Method == http.MethodPut {
		var config Config
		if c.ShouldBindJSON(&config) != nil {
			response.BadRequest(c, "Invalid monitor configuration")
			return
		}
		err = r.store.Configure(ctx, id, config)
		if errors.Is(err, ErrInvalid) {
			response.BadRequest(c, err.Error())
			return
		}
		if err != nil {
			response.Error(c, 500, "Monitor configuration could not be saved")
			return
		}
		response.Success(c, gin.H{"saved": true})
		return
	}
	hours, page := 24, 1
	if value := c.Query("hours"); value != "" {
		hours, _ = strconv.Atoi(value)
	}
	if value := c.Query("page"); value != "" {
		page, _ = strconv.Atoi(value)
	}
	var until time.Time
	if value := c.Query("until"); value != "" {
		until, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			response.BadRequest(c, "Invalid history cutoff")
			return
		}
	}
	report, err := r.store.QueryAt(ctx, id, hours, page, until)
	if errors.Is(err, ErrInvalid) {
		response.BadRequest(c, err.Error())
		return
	}
	if err != nil {
		response.Error(c, 500, "Monitor history unavailable")
		return
	}
	response.Success(c, report)
}

func (r *Runner) readConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	settings, err := r.store.ReadConfig(ctx, id)
	if errors.Is(err, ErrInvalid) {
		response.BadRequest(c, err.Error())
		return
	}
	if err != nil {
		response.Error(c, 500, "Monitor configuration unavailable")
		return
	}
	response.Success(c, settings)
}
