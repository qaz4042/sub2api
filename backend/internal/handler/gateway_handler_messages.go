package handler

import (
	"github.com/gin-gonic/gin"
)

// Messages handles Claude API compatible messages endpoint
// POST /v1/messages
func (h *GatewayHandler) Messages(c *gin.Context) {
	reqCtx, ok := h.prepareGatewayRequestContext(c, "handler.gateway.messages", h.errorResponse)
	if !ok {
		return
	}
	defer func() {
		h.maybeLogCompatibilityFallbackMetrics(reqCtx.Log)
	}()

	req, ok := h.prepareGatewayMessageRequest(c, reqCtx)
	if !ok {
		return
	}
	if req.ReleaseUserSlot != nil {
		defer req.ReleaseUserSlot()
	}

	h.runGatewayMessages(c, req)
}
