package handler

import (
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *GatewayHandler) runGatewayGeminiMessages(c *gin.Context, req *gatewayMessageRequest) {
	fs := NewFailoverState(h.maxAccountSwitchesGemini, req.HasBoundSession)
	h.markGatewaySingleAccountRetry(c, req.Ctx.APIKey.GroupID)

	for {
		selection, err := h.selectGatewayGeminiMessageAccount(c, req, fs)
		if err != nil {
			if h.handleGatewayMessageSelectionError(c, req, fs, err, req.Ctx.APIKey.GroupID, service.PlatformGemini, false) {
				continue
			}
			return
		}

		if h.forwardGatewayGeminiMessageAttempt(c, req, fs, selection) != gatewayAttemptContinue {
			return
		}
	}
}

func (h *GatewayHandler) selectGatewayGeminiMessageAccount(
	c *gin.Context,
	req *gatewayMessageRequest,
	fs *FailoverState,
) (*service.AccountSelectionResult, error) {
	return h.gatewayService.SelectAccountWithLoadAwareness(
		c.Request.Context(),
		req.Ctx.APIKey.GroupID,
		req.SessionKey,
		req.Model,
		fs.FailedAccountIDs,
		"",
		int64(0),
	)
}

func (h *GatewayHandler) forwardGatewayGeminiMessageAttempt(
	c *gin.Context,
	req *gatewayMessageRequest,
	fs *FailoverState,
	selection *service.AccountSelectionResult,
) gatewayAttemptOutcome {
	account := selection.Account
	setOpsSelectedAccount(c, account.ID, account.Platform)

	if h.handleGatewayMessageIntercept(c, req, selection) {
		return gatewayAttemptDone
	}

	accountRelease, ok := h.acquireGatewayAccountSlot(c, req.Ctx.Log, selection, gatewayAccountSlotOptions{
		GroupID:          req.Ctx.APIKey.GroupID,
		SessionKey:       req.SessionKey,
		ReqStream:        req.Stream,
		StreamStarted:    &req.StreamStarted,
		LogPrefix:        "gateway",
		WriteErr:         h.errorResponse,
		TrackWaitQueue:   true,
		BindStickyOnWait: true,
	})
	if !ok {
		return gatewayAttemptDone
	}

	writerSizeBeforeForward := c.Writer.Size()
	result, err := h.forwardGatewayGeminiMessage(c, req, fs, account)
	accountRelease()
	if err != nil {
		return h.handleGatewayGeminiForwardError(c, req, fs, account, writerSizeBeforeForward, err)
	}

	h.finalizeGatewayMessageSuccess(c, req, fs, account, req.Parsed, result, req.Ctx.APIKey, req.Subscription, req.Body)
	return gatewayAttemptDone
}

func (h *GatewayHandler) forwardGatewayGeminiMessage(
	c *gin.Context,
	req *gatewayMessageRequest,
	fs *FailoverState,
	account *service.Account,
) (*service.ForwardResult, error) {
	requestCtx := c.Request.Context()
	if fs.SwitchCount > 0 {
		requestCtx = service.WithAccountSwitchCount(requestCtx, fs.SwitchCount, h.metadataBridgeEnabled())
	}
	if account.Platform == service.PlatformAntigravity {
		return h.antigravityGatewayService.ForwardGemini(
			requestCtx,
			c,
			account,
			req.Model,
			"generateContent",
			req.Stream,
			req.Body,
			req.HasBoundSession,
			service.WithForwardGeminiSession(derefGroupID(req.Ctx.APIKey.GroupID), req.SessionKey),
		)
	}
	return h.geminiCompatService.Forward(requestCtx, c, account, req.Body)
}

func (h *GatewayHandler) handleGatewayGeminiForwardError(
	c *gin.Context,
	req *gatewayMessageRequest,
	fs *FailoverState,
	account *service.Account,
	writerSizeBeforeForward int,
	err error,
) gatewayAttemptOutcome {
	var failoverErr *service.UpstreamFailoverError
	if errors.As(err, &failoverErr) {
		if c.Writer.Size() != writerSizeBeforeForward {
			h.handleFailoverExhausted(c, failoverErr, service.PlatformGemini, true)
			return gatewayAttemptDone
		}
		switch fs.HandleFailoverError(c.Request.Context(), h.gatewayService, account.ID, account.Platform, failoverErr) {
		case FailoverContinue:
			return gatewayAttemptContinue
		case FailoverExhausted:
			h.handleFailoverExhausted(c, fs.LastFailoverErr, service.PlatformGemini, req.StreamStarted)
		case FailoverCanceled:
			return gatewayAttemptDone
		}
		return gatewayAttemptDone
	}

	h.logGatewayMessageForwardError(c, req, account, writerSizeBeforeForward, err)
	return gatewayAttemptDone
}
