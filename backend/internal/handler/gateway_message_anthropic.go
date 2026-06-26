package handler

import "github.com/gin-gonic/gin"

func (h *GatewayHandler) runGatewayAnthropicMessages(c *gin.Context, req *gatewayMessageRequest) {
	route := &gatewayAnthropicRoute{
		APIKey:       req.Ctx.APIKey,
		Subscription: req.Subscription,
	}
	if req.Ctx.APIKey.Group != nil {
		route.FallbackGroupID = req.Ctx.APIKey.Group.FallbackGroupIDOnInvalidRequest
	}
	h.markGatewaySingleAccountRetry(c, route.APIKey.GroupID)

	for {
		fs := NewFailoverState(h.maxAccountSwitches, req.HasBoundSession)
		if h.runGatewayAnthropicFailoverLoop(c, req, route, fs) != gatewayAttemptRetryFallback {
			return
		}
	}
}

func (h *GatewayHandler) runGatewayAnthropicFailoverLoop(
	c *gin.Context,
	req *gatewayMessageRequest,
	route *gatewayAnthropicRoute,
	fs *FailoverState,
) gatewayAttemptOutcome {
	for {
		attempt, ok := h.prepareGatewayAnthropicMessageAttempt(c, req, route, fs)
		if !ok {
			return gatewayAttemptDone
		}
		if attempt == nil {
			continue
		}

		outcome := h.forwardGatewayAnthropicMessageAttempt(c, req, route, fs, attempt)
		if outcome != gatewayAttemptContinue {
			return outcome
		}
	}
}
