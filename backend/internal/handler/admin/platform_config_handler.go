package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PlatformConfigHandler struct {
	service *service.PlatformConfigService
}

func NewPlatformConfigHandler(service *service.PlatformConfigService) *PlatformConfigHandler {
	return &PlatformConfigHandler{service: service}
}

type updatePlatformConfigRequest struct {
	Enabled *bool `json:"enabled"`
}

func (h *PlatformConfigHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *PlatformConfigHandler) Update(c *gin.Context) {
	var req updatePlatformConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "enabled is required")
		return
	}
	item, err := h.service.SetEnabled(c.Request.Context(), c.Param("key"), *req.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}
