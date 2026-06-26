package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *GatewayHandler) prepareGatewayAnthropicAttemptBody(
	c *gin.Context,
	req *gatewayMessageRequest,
	route *gatewayAnthropicRoute,
	attempt *gatewayMessageAttempt,
) bool {
	attempt.QueueRelease = h.acquireGatewayUserMessageQueue(c, req, attempt.Account, attempt.Parsed)
	attempt.Parsed.OnUpstreamAccepted = attempt.QueueRelease

	if !h.applyGatewayAnthropicChannelMapping(c, req, attempt) {
		return false
	}
	if !h.applyGatewayAnthropicBedrockCompat(c, route, attempt) {
		return false
	}
	attempt.Body = attempt.Parsed.Body.Bytes()
	return true
}

func (h *GatewayHandler) applyGatewayAnthropicChannelMapping(
	c *gin.Context,
	req *gatewayMessageRequest,
	attempt *gatewayMessageAttempt,
) bool {
	if !req.ChannelMapping.Mapped {
		return true
	}
	attempt.Parsed.Model = req.ChannelMapping.MappedModel
	if err := attempt.Parsed.ReplaceBody(h.gatewayService.ReplaceModelInBody(attempt.Parsed.Body.Bytes(), req.ChannelMapping.MappedModel)); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return false
	}
	return true
}

func (h *GatewayHandler) applyGatewayAnthropicBedrockCompat(
	c *gin.Context,
	route *gatewayAnthropicRoute,
	attempt *gatewayMessageAttempt,
) bool {
	body := h.gatewayService.ApplyBedrockCCCompat(c, attempt.Parsed.Body.Bytes(), attempt.Parsed.Model, attempt.Account, route.APIKey.GroupID)
	if err := attempt.Parsed.ReplaceBody(body); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return false
	}
	return true
}

func (h *GatewayHandler) acquireGatewayUserMessageQueue(
	c *gin.Context,
	req *gatewayMessageRequest,
	account *service.Account,
	parsedReq *service.ParsedRequest,
) func() {
	var queueRelease func()
	umqMode := h.getUserMsgQueueMode(account, parsedReq)

	switch umqMode {
	case config.UMQModeSerialize:
		release, qErr := h.userMsgQueueHelper.AcquireWithWait(
			c,
			account.ID,
			account.GetBaseRPM(),
			req.Stream,
			&req.StreamStarted,
			h.cfg.Gateway.UserMessageQueue.WaitTimeout(),
			req.Ctx.Log,
		)
		if qErr != nil {
			req.Ctx.Log.Warn("gateway.umq_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(qErr))
		} else {
			queueRelease = release
		}

	case config.UMQModeThrottle:
		if tErr := h.userMsgQueueHelper.ThrottleWithPing(
			c,
			account.ID,
			account.GetBaseRPM(),
			req.Stream,
			&req.StreamStarted,
			h.cfg.Gateway.UserMessageQueue.WaitTimeout(),
			req.Ctx.Log,
		); tErr != nil {
			req.Ctx.Log.Warn("gateway.umq_throttle_failed", zap.Int64("account_id", account.ID), zap.Error(tErr))
		}

	default:
		if umqMode != "" {
			req.Ctx.Log.Warn("gateway.umq_unknown_mode", zap.String("mode", umqMode), zap.Int64("account_id", account.ID))
		}
	}

	return wrapReleaseOnDone(c.Request.Context(), queueRelease)
}
