package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *GatewayHandler) runGatewayMessages(c *gin.Context, req *gatewayMessageRequest) {
	if req.Platform == service.PlatformGemini {
		h.runGatewayGeminiMessages(c, req)
		return
	}
	h.runGatewayAnthropicMessages(c, req)
}

func (h *GatewayHandler) markGatewaySingleAccountRetry(c *gin.Context, groupID *int64) {
	if !h.gatewayService.IsSingleAntigravityAccountGroup(c.Request.Context(), groupID) {
		return
	}
	ctx := service.WithSingleAccountRetry(c.Request.Context(), true, h.metadataBridgeEnabled())
	c.Request = c.Request.WithContext(ctx)
}
