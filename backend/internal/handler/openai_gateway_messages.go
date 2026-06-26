package handler

import (
	"time"

	"github.com/gin-gonic/gin"
)

// Messages handles Anthropic Messages API requests routed to OpenAI platform.
func (h *OpenAIGatewayHandler) Messages(c *gin.Context) {
	streamStarted := false
	defer h.recoverAnthropicMessagesPanic(c, &streamStarted)

	req, ok := h.prepareOpenAIAnthropicMessagesRequest(c, &streamStarted, time.Now())
	if !ok {
		return
	}
	defer req.release()

	h.runOpenAIAnthropicMessagesRoute(c, req, &streamStarted)
}
