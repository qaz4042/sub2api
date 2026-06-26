package handler

import (
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) selectOpenAIResponsesAccount(
	c *gin.Context,
	req *openAIResponsesRequest,
	state *openAIResponsesRouteState,
	streamStarted *bool,
) (*service.AccountSelectionResult, bool) {
	req.Log.Debug("openai.account_selecting", zap.Int("excluded_account_count", len(state.FailedAccountIDs)))
	selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
		c.Request.Context(),
		req.APIKey.GroupID,
		req.PreviousResponseID,
		req.SessionHash,
		req.Model,
		state.FailedAccountIDs,
		service.OpenAIUpstreamTransportAny,
		service.OpenAIEndpointCapabilityChatCompletions,
		req.RequireCompact,
	)
	if err != nil {
		return nil, h.handleOpenAIResponsesSelectionError(c, req, state, err, streamStarted)
	}
	if selection == nil || selection.Account == nil {
		markOpsRoutingCapacityLimited(c)
		h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", streamStartedValue(streamStarted))
		return nil, false
	}
	if req.PreviousResponseID != "" {
		req.Log.Debug("openai.account_selected_with_previous_response_id", zap.Int64("account_id", selection.Account.ID))
	}
	req.Log.Debug("openai.account_schedule_decision",
		zap.String("layer", scheduleDecision.Layer),
		zap.Bool("sticky_previous_hit", scheduleDecision.StickyPreviousHit),
		zap.Bool("sticky_session_hit", scheduleDecision.StickySessionHit),
		zap.Int("candidate_count", scheduleDecision.CandidateCount),
		zap.Int("top_k", scheduleDecision.TopK),
		zap.Int64("latency_ms", scheduleDecision.LatencyMs),
		zap.Float64("load_skew", scheduleDecision.LoadSkew),
	)
	return selection, true
}

func (h *OpenAIGatewayHandler) handleOpenAIResponsesSelectionError(
	c *gin.Context,
	req *openAIResponsesRequest,
	state *openAIResponsesRouteState,
	err error,
	streamStarted *bool,
) bool {
	req.Log.Warn("openai.account_select_failed",
		zap.Error(err),
		zap.Int("excluded_account_count", len(state.FailedAccountIDs)),
	)
	if len(state.FailedAccountIDs) == 0 {
		markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
		if errors.Is(err, service.ErrNoAvailableCompactAccounts) {
			h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "compact_not_supported", "No available OpenAI accounts support /responses/compact", streamStartedValue(streamStarted))
			return false
		}
		h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable", streamStartedValue(streamStarted))
		return false
	}
	if state.LastFailoverErr != nil {
		h.handleFailoverExhausted(c, state.LastFailoverErr, streamStartedValue(streamStarted))
	} else {
		h.handleFailoverExhaustedSimple(c, 502, streamStartedValue(streamStarted))
	}
	return false
}
