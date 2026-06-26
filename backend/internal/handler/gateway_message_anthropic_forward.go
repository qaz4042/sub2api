package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *GatewayHandler) forwardGatewayAnthropicMessageAttempt(
	c *gin.Context,
	req *gatewayMessageRequest,
	route *gatewayAnthropicRoute,
	fs *FailoverState,
	attempt *gatewayMessageAttempt,
) gatewayAttemptOutcome {
	c.Set("parsed_request", attempt.Parsed)
	attempt.WriterSizeBeforeForward = c.Writer.Size()
	result, err := h.forwardGatewayAnthropicMessage(c, req, fs, attempt)
	attempt.Result = result
	attempt.release()

	if err != nil {
		return h.handleGatewayAnthropicForwardError(c, req, route, fs, attempt, err)
	}

	h.bindGatewayMessageStickySession(c, req, route, attempt.Account)
	h.finalizeGatewayMessageSuccess(c, req, fs, attempt.Account, attempt.Parsed, result, route.APIKey, route.Subscription, attempt.Body)
	return gatewayAttemptDone
}

func (h *GatewayHandler) forwardGatewayAnthropicMessage(
	c *gin.Context,
	req *gatewayMessageRequest,
	fs *FailoverState,
	attempt *gatewayMessageAttempt,
) (*service.ForwardResult, error) {
	requestCtx := c.Request.Context()
	if fs.SwitchCount > 0 {
		requestCtx = service.WithAccountSwitchCount(requestCtx, fs.SwitchCount, h.metadataBridgeEnabled())
	}
	if attempt.Account.Platform == service.PlatformAntigravity && attempt.Account.Type != service.AccountTypeAPIKey {
		return h.antigravityGatewayService.Forward(requestCtx, c, attempt.Account, attempt.Body, req.HasBoundSession)
	}
	return h.gatewayService.Forward(requestCtx, c, attempt.Account, attempt.Parsed)
}
