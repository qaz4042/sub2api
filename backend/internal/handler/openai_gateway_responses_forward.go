package handler

import (
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) forwardOpenAIResponsesAttempt(
	c *gin.Context,
	req *openAIResponsesRequest,
	state *openAIResponsesRouteState,
	account *service.Account,
	accountRelease func(),
	streamStarted *bool,
) gatewayAttemptOutcome {
	service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(req.RoutingStart).Milliseconds())
	forwardStart := time.Now()
	writerSizeBeforeForward := c.Writer.Size()
	result, err := func() (*service.OpenAIForwardResult, error) {
		defer func() {
			if accountRelease != nil {
				accountRelease()
			}
		}()
		return h.gatewayService.Forward(c.Request.Context(), c, account, req.ForwardBody)
	}()

	h.recordOpenAIResponsesCyberPolicy(c, req, account, err != nil)
	h.recordOpenAIResponsesLatency(c, forwardStart, result, err)
	if err != nil {
		return h.handleOpenAIResponsesForwardError(c, req, state, account, writerSizeBeforeForward, result, err, streamStarted)
	}

	h.finalizeOpenAIResponsesSuccess(c, req, state, account, result)
	return gatewayAttemptDone
}

func (h *OpenAIGatewayHandler) handleOpenAIResponsesForwardError(
	c *gin.Context,
	req *openAIResponsesRequest,
	state *openAIResponsesRouteState,
	account *service.Account,
	writerSizeBeforeForward int,
	result *service.OpenAIForwardResult,
	err error,
	streamStarted *bool,
) gatewayAttemptOutcome {
	if result != nil && result.ImageCount > 0 {
		req.Log.Warn("openai.forward_partial_error_with_image_result",
			zap.Int64("account_id", account.ID),
			zap.Int("image_count", result.ImageCount),
			zap.Error(err),
		)
		return gatewayAttemptDone
	}

	var failoverErr *service.UpstreamFailoverError
	if errors.As(err, &failoverErr) {
		return h.handleOpenAIResponsesFailoverError(c, req, state, account, writerSizeBeforeForward, failoverErr, streamStarted)
	}

	h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
	upstreamErrorAlreadyCommunicated := openAIForwardErrorAlreadyCommunicated(c, writerSizeBeforeForward, err)
	wroteFallback := false
	if !upstreamErrorAlreadyCommunicated {
		wroteFallback = h.ensureForwardErrorResponse(c, streamStartedValue(streamStarted))
	}
	fields := []zap.Field{
		zap.Int64("account_id", account.ID),
		zap.Bool("fallback_error_response_written", wroteFallback),
		zap.Bool("upstream_error_response_already_written", upstreamErrorAlreadyCommunicated),
		zap.Error(err),
	}
	if shouldLogOpenAIForwardFailureAsWarn(c, wroteFallback) {
		req.Log.Warn("openai.forward_failed", fields...)
		return gatewayAttemptDone
	}
	req.Log.Error("openai.forward_failed", fields...)
	return gatewayAttemptDone
}

func (h *OpenAIGatewayHandler) handleOpenAIResponsesFailoverError(
	c *gin.Context,
	req *openAIResponsesRequest,
	state *openAIResponsesRouteState,
	account *service.Account,
	writerSizeBeforeForward int,
	failoverErr *service.UpstreamFailoverError,
	streamStarted *bool,
) gatewayAttemptOutcome {
	if c.Writer.Size() != writerSizeBeforeForward {
		h.handleFailoverExhausted(c, failoverErr, true)
		return gatewayAttemptDone
	}
	h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
	if h.retryOpenAIResponsesSameAccount(c, req, state, account, failoverErr) {
		return gatewayAttemptContinue
	}
	h.gatewayService.RecordOpenAIAccountSwitch()
	state.FailedAccountIDs[account.ID] = struct{}{}
	state.LastFailoverErr = failoverErr
	if state.SwitchCount >= state.MaxAccountSwitches {
		h.handleFailoverExhausted(c, failoverErr, streamStartedValue(streamStarted))
		return gatewayAttemptDone
	}
	state.SwitchCount++
	if h.gatewayService.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, state.SwitchCount) {
		h.handleFailoverExhausted(c, failoverErr, streamStartedValue(streamStarted))
		return gatewayAttemptDone
	}
	req.Log.Warn("openai.upstream_failover_switching",
		zap.Int64("account_id", account.ID),
		zap.Int("upstream_status", failoverErr.StatusCode),
		zap.Int("switch_count", state.SwitchCount),
		zap.Int("max_switches", state.MaxAccountSwitches),
	)
	return gatewayAttemptContinue
}

func (h *OpenAIGatewayHandler) retryOpenAIResponsesSameAccount(
	c *gin.Context,
	req *openAIResponsesRequest,
	state *openAIResponsesRouteState,
	account *service.Account,
	failoverErr *service.UpstreamFailoverError,
) bool {
	if !failoverErr.RetryableOnSameAccount {
		return false
	}
	retryLimit := account.GetPoolModeRetryCount()
	if state.SameAccountRetryCount[account.ID] >= retryLimit {
		return false
	}
	state.SameAccountRetryCount[account.ID]++
	req.Log.Warn("openai.pool_mode_same_account_retry",
		zap.Int64("account_id", account.ID),
		zap.Int("upstream_status", failoverErr.StatusCode),
		zap.Int("retry_limit", retryLimit),
		zap.Int("retry_count", state.SameAccountRetryCount[account.ID]),
	)
	select {
	case <-c.Request.Context().Done():
		return false
	case <-time.After(sameAccountRetryDelay):
		return true
	}
}
