package admin

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type openAIProxyLatencyService interface {
	GetOpenAIProxyLatency(context.Context, int64) (*service.OpenAIProxyLatency, error)
	TestOpenAIProxyLatency(context.Context, int64) (*service.OpenAIProxyLatency, error)
}

// RegisterOpenAILatency is mounted inside the existing admin-authenticated group.
func (h *ProxyHandler) RegisterOpenAILatency(routes *gin.RouterGroup) {
	routes.GET("/:id/openai-latency", h.openAILatency)
	routes.POST("/:id/openai-latency", h.openAILatency)
}

func (h *ProxyHandler) openAILatency(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}
	svc, ok := h.adminService.(openAIProxyLatencyService)
	if !ok {
		response.Error(c, http.StatusServiceUnavailable, "OpenAI proxy latency is unavailable")
		return
	}
	var result *service.OpenAIProxyLatency
	if c.Request.Method == http.MethodPost {
		result, err = svc.TestOpenAIProxyLatency(c.Request.Context(), id)
	} else {
		result, err = svc.GetOpenAIProxyLatency(c.Request.Context(), id)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
