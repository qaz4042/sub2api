package handler

import (
	"time"

	"github.com/gin-gonic/gin"
)

func (h *OpenAIGatewayHandler) Responses(c *gin.Context) {
	// 局部兜底：确保该 handler 内部任何 panic 都不会击穿到进程级。
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	compactStartedAt := time.Now()
	defer h.logOpenAIRemoteCompactOutcome(c, compactStartedAt)
	setOpenAIClientTransportHTTP(c)

	requestStart := time.Now()
	req, ok := h.prepareOpenAIResponsesRequest(c, &streamStarted, requestStart)
	if !ok {
		return
	}
	defer req.release()

	h.runOpenAIResponsesRoute(c, req, &streamStarted)
}
