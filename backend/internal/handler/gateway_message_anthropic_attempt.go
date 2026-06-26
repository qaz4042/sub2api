package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *GatewayHandler) prepareGatewayAnthropicMessageAttempt(
	c *gin.Context,
	req *gatewayMessageRequest,
	route *gatewayAnthropicRoute,
	fs *FailoverState,
) (*gatewayMessageAttempt, bool) {
	attemptParsedReq, err := req.Parsed.CloneForBody(req.Body)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return nil, false
	}

	selection, outcome := h.selectGatewayAnthropicMessageAccount(c, req, route, fs)
	switch outcome {
	case gatewaySelectionRetry:
		return nil, true
	case gatewaySelectionDone:
		return nil, false
	}
	account := selection.Account
	setOpsSelectedAccount(c, account.ID, account.Platform)
	h.logGatewayMessageAccountSelected(req, account, selection)

	if h.handleGatewayMessageIntercept(c, req, selection) {
		return nil, false
	}

	accountRelease, ok := h.acquireGatewayAnthropicAttemptSlot(c, req, route, selection)
	if !ok {
		return nil, false
	}

	attempt := &gatewayMessageAttempt{
		Account:        account,
		Selection:      selection,
		Parsed:         attemptParsedReq,
		AccountRelease: accountRelease,
	}
	if !h.prepareGatewayAnthropicAttemptBody(c, req, route, attempt) {
		attempt.release()
		return nil, false
	}
	return attempt, true
}

func (h *GatewayHandler) selectGatewayAnthropicMessageAccount(
	c *gin.Context,
	req *gatewayMessageRequest,
	route *gatewayAnthropicRoute,
	fs *FailoverState,
) (*service.AccountSelectionResult, gatewaySelectionOutcome) {
	req.Ctx.Log.Info("sticky.selecting_account",
		zap.String("session_key", req.SessionKey),
		zap.Int64("sticky_bound_account_id", req.SessionBoundAccount),
		zap.Bool("has_bound_session", req.HasBoundSession),
		zap.Int("failed_account_count", len(fs.FailedAccountIDs)),
	)

	selection, err := h.gatewayService.SelectAccountWithLoadAwareness(
		c.Request.Context(),
		route.APIKey.GroupID,
		req.SessionKey,
		req.Model,
		fs.FailedAccountIDs,
		req.Parsed.MetadataUserID,
		req.Ctx.Subject.UserID,
	)
	if err == nil {
		return selection, gatewaySelectionSelected
	}
	if h.handleGatewayMessageSelectionError(c, req, fs, err, route.APIKey.GroupID, req.Platform, route.FallbackUsed) {
		return nil, gatewaySelectionRetry
	}
	return nil, gatewaySelectionDone
}

func (h *GatewayHandler) acquireGatewayAnthropicAttemptSlot(
	c *gin.Context,
	req *gatewayMessageRequest,
	route *gatewayAnthropicRoute,
	selection *service.AccountSelectionResult,
) (func(), bool) {
	return h.acquireGatewayAccountSlot(c, req.Ctx.Log, selection, gatewayAccountSlotOptions{
		GroupID:          route.APIKey.GroupID,
		SessionKey:       req.SessionKey,
		ReqStream:        req.Stream,
		StreamStarted:    &req.StreamStarted,
		LogPrefix:        "gateway",
		WriteErr:         h.errorResponse,
		TrackWaitQueue:   true,
		BindStickyOnWait: true,
	})
}
