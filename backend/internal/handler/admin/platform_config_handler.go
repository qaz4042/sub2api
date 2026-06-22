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

type createPlatformConfigRequest struct {
	Key         string `json:"key" binding:"required"`
	Label       string `json:"label" binding:"required"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	SortOrder   int    `json:"sort_order"`
}

type updatePlatformConfigRequest struct {
	Label       *string `json:"label"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
	SortOrder   *int    `json:"sort_order"`
}

func (h *PlatformConfigHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *PlatformConfigHandler) Create(c *gin.Context) {
	var req createPlatformConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	item, err := h.service.Create(c.Request.Context(), service.PlatformConfigInput{
		Key:         req.Key,
		Label:       req.Label,
		Description: req.Description,
		Enabled:     req.Enabled,
		SortOrder:   req.SortOrder,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *PlatformConfigHandler) Update(c *gin.Context) {
	var req updatePlatformConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	item, err := h.service.Update(c.Request.Context(), c.Param("key"), service.PlatformConfigUpdate{
		Label:       req.Label,
		Description: req.Description,
		Enabled:     req.Enabled,
		SortOrder:   req.SortOrder,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *PlatformConfigHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("key")); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
