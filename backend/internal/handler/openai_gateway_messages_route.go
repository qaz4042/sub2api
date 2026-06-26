package handler

import (
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type openAIAnthropicMessagesRouteState struct {
	SwitchCount           int
	MaxAccountSwitches    int
	FailedAccountIDs      map[int64]struct{}
	SameAccountRetryCount map[int64]int
	LastFailoverErr       *service.UpstreamFailoverError
}

func (h *OpenAIGatewayHandler) runOpenAIAnthropicMessagesRoute(
	c *gin.Context,
	req *openAIAnthropicMessagesRequest,
	streamStarted *bool,
) {
	state := &openAIAnthropicMessagesRouteState{
		MaxAccountSwitches:    h.maxAccountSwitches,
		FailedAccountIDs:      make(map[int64]struct{}),
		SameAccountRetryCount: make(map[int64]int),
	}

	for {
		selection, ok := h.selectOpenAIAnthropicMessagesAccount(c, req, state, streamStarted)
		if !ok {
			return
		}

		account := selection.Account
		req.SessionHash = ensureOpenAIPoolModeSessionHash(req.SessionHash, account)
		req.Log.Debug("openai_messages.account_selected", zap.Int64("account_id", account.ID), zap.String("account_name", account.Name))
		setOpsSelectedAccount(c, account.ID, account.Platform)

		accountRelease, acquired := h.acquireResponsesAccountSlot(c, req.APIKey.GroupID, req.SessionHash, selection, req.Stream, streamStarted, req.Log)
		if !acquired {
			return
		}

		if h.forwardOpenAIAnthropicMessagesAttempt(c, req, state, account, accountRelease, streamStarted) != gatewayAttemptContinue {
			return
		}
	}
}

func (h *OpenAIGatewayHandler) selectOpenAIAnthropicMessagesAccount(
	c *gin.Context,
	req *openAIAnthropicMessagesRequest,
	state *openAIAnthropicMessagesRouteState,
	streamStarted *bool,
) (*service.AccountSelectionResult, bool) {
	currentRoutingModel := req.RoutingModel
	if req.EffectiveMapped != "" {
		currentRoutingModel = req.EffectiveMapped
	}
	req.Log.Debug("openai_messages.account_selecting", zap.Int("excluded_account_count", len(state.FailedAccountIDs)))
	selection, _, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
		c.Request.Context(),
		req.APIKey.GroupID,
		"",
		req.SessionHash,
		currentRoutingModel,
		state.FailedAccountIDs,
		service.OpenAIUpstreamTransportAny,
		service.OpenAIEndpointCapabilityChatCompletions,
		false,
	)
	if err != nil {
		return nil, h.handleOpenAIAnthropicMessagesSelectionError(c, req, state, err, streamStarted)
	}
	if selection == nil || selection.Account == nil {
		markOpsRoutingCapacityLimited(c)
		h.anthropicStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", streamStartedValue(streamStarted))
		return nil, false
	}
	return selection, true
}

func (h *OpenAIGatewayHandler) handleOpenAIAnthropicMessagesSelectionError(
	c *gin.Context,
	req *openAIAnthropicMessagesRequest,
	state *openAIAnthropicMessagesRouteState,
	err error,
	streamStarted *bool,
) bool {
	req.Log.Warn("openai_messages.account_select_failed",
		zap.Error(err),
		zap.Int("excluded_account_count", len(state.FailedAccountIDs)),
	)
	if len(state.FailedAccountIDs) == 0 {
		markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
		h.anthropicStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable", streamStartedValue(streamStarted))
		return false
	}
	if state.LastFailoverErr != nil {
		h.handleAnthropicFailoverExhausted(c, state.LastFailoverErr, streamStartedValue(streamStarted))
	} else {
		h.anthropicStreamingAwareError(c, http.StatusBadGateway, "api_error", "Upstream request failed", streamStartedValue(streamStarted))
	}
	return false
}

func (h *OpenAIGatewayHandler) handleOpenAIAnthropicMessagesForwardError(
	c *gin.Context,
	req *openAIAnthropicMessagesRequest,
	state *openAIAnthropicMessagesRouteState,
	account *service.Account,
	writerSizeBeforeForward int,
	result *service.OpenAIForwardResult,
	err error,
	streamStarted *bool,
) gatewayAttemptOutcome {
	if result != nil && result.ImageCount > 0 {
		req.Log.Warn("openai_messages.forward_partial_error_with_image_result",
			zap.Int64("account_id", account.ID),
			zap.Int("image_count", result.ImageCount),
			zap.Error(err),
		)
		return gatewayAttemptDone
	}

	var failoverErr *service.UpstreamFailoverError
	if errors.As(err, &failoverErr) {
		return h.handleOpenAIAnthropicMessagesFailoverError(c, req, state, account, writerSizeBeforeForward, failoverErr, streamStarted)
	}

	if result != nil && result.ClientDisconnect {
		req.Log.Info("openai_messages.client_disconnected",
			zap.Int64("account_id", account.ID),
			zap.Error(err),
		)
		return gatewayAttemptDone
	}

	h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
	wroteFallback := h.ensureAnthropicErrorResponse(c, streamStartedValue(streamStarted))
	req.Log.Warn("openai_messages.forward_failed",
		zap.Int64("account_id", account.ID),
		zap.Bool("fallback_error_response_written", wroteFallback),
		zap.Error(err),
	)
	return gatewayAttemptDone
}

func (h *OpenAIGatewayHandler) handleOpenAIAnthropicMessagesFailoverError(
	c *gin.Context,
	req *openAIAnthropicMessagesRequest,
	state *openAIAnthropicMessagesRouteState,
	account *service.Account,
	writerSizeBeforeForward int,
	failoverErr *service.UpstreamFailoverError,
	streamStarted *bool,
) gatewayAttemptOutcome {
	if c.Writer.Size() != writerSizeBeforeForward {
		h.handleAnthropicFailoverExhausted(c, failoverErr, true)
		return gatewayAttemptDone
	}
	h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
	if h.retryOpenAIAnthropicMessagesSameAccount(c, req, state, account, failoverErr) {
		return gatewayAttemptContinue
	}
	h.gatewayService.RecordOpenAIAccountSwitch()
	state.FailedAccountIDs[account.ID] = struct{}{}
	state.LastFailoverErr = failoverErr
	if state.SwitchCount >= state.MaxAccountSwitches {
		h.handleAnthropicFailoverExhausted(c, failoverErr, streamStartedValue(streamStarted))
		return gatewayAttemptDone
	}
	state.SwitchCount++
	if h.gatewayService.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, state.SwitchCount) {
		h.handleAnthropicFailoverExhausted(c, failoverErr, streamStartedValue(streamStarted))
		return gatewayAttemptDone
	}
	req.Log.Warn("openai_messages.upstream_failover_switching",
		zap.Int64("account_id", account.ID),
		zap.Int("upstream_status", failoverErr.StatusCode),
		zap.Int("switch_count", state.SwitchCount),
		zap.Int("max_switches", state.MaxAccountSwitches),
	)
	return gatewayAttemptContinue
}
