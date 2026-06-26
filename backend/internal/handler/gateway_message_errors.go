package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *GatewayHandler) handleGatewayAnthropicForwardError(
	c *gin.Context,
	req *gatewayMessageRequest,
	route *gatewayAnthropicRoute,
	fs *FailoverState,
	attempt *gatewayMessageAttempt,
	err error,
) gatewayAttemptOutcome {
	var betaBlockedErr *service.BetaBlockedError
	if errors.As(err, &betaBlockedErr) {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", betaBlockedErr.Message)
		return gatewayAttemptDone
	}

	var promptTooLongErr *service.PromptTooLongError
	if errors.As(err, &promptTooLongErr) {
		return h.handleGatewayMessagePromptTooLong(c, req, route, attempt.Account, promptTooLongErr)
	}

	var failoverErr *service.UpstreamFailoverError
	if errors.As(err, &failoverErr) {
		if c.Writer.Size() != attempt.WriterSizeBeforeForward {
			h.handleFailoverExhausted(c, failoverErr, attempt.Account.Platform, true)
			return gatewayAttemptDone
		}
		switch fs.HandleFailoverError(c.Request.Context(), h.gatewayService, attempt.Account.ID, attempt.Account.Platform, failoverErr) {
		case FailoverContinue:
			return gatewayAttemptContinue
		case FailoverExhausted:
			h.handleFailoverExhausted(c, fs.LastFailoverErr, attempt.Account.Platform, req.StreamStarted)
		case FailoverCanceled:
			return gatewayAttemptDone
		}
		return gatewayAttemptDone
	}

	h.logGatewayMessageForwardError(c, req, attempt.Account, attempt.WriterSizeBeforeForward, err)
	return gatewayAttemptDone
}

func (h *GatewayHandler) handleGatewayMessagePromptTooLong(
	c *gin.Context,
	req *gatewayMessageRequest,
	route *gatewayAnthropicRoute,
	account *service.Account,
	promptTooLongErr *service.PromptTooLongError,
) gatewayAttemptOutcome {
	req.Ctx.Log.Warn("gateway.prompt_too_long_from_antigravity",
		zap.Any("current_group_id", route.APIKey.GroupID),
		zap.Any("fallback_group_id", route.FallbackGroupID),
		zap.Bool("fallback_used", route.FallbackUsed),
	)

	if route.FallbackUsed || route.FallbackGroupID == nil || *route.FallbackGroupID <= 0 {
		h.writeGatewayPromptTooLong(c, account, promptTooLongErr)
		return gatewayAttemptDone
	}

	fallbackAPIKey, ok := h.resolveGatewayMessageFallbackAPIKey(c, req, route, account, promptTooLongErr)
	if !ok {
		return gatewayAttemptDone
	}

	ctx := context.WithValue(c.Request.Context(), ctxkey.ForcePlatform, "")
	c.Request = c.Request.WithContext(ctx)
	route.APIKey = fallbackAPIKey
	route.Subscription = nil
	route.FallbackUsed = true
	return gatewayAttemptRetryFallback
}

func (h *GatewayHandler) resolveGatewayMessageFallbackAPIKey(
	c *gin.Context,
	req *gatewayMessageRequest,
	route *gatewayAnthropicRoute,
	account *service.Account,
	promptTooLongErr *service.PromptTooLongError,
) (*service.APIKey, bool) {
	fallbackGroup, err := h.gatewayService.ResolveGroupByID(c.Request.Context(), *route.FallbackGroupID)
	if err != nil {
		req.Ctx.Log.Warn("gateway.resolve_fallback_group_failed", zap.Int64("fallback_group_id", *route.FallbackGroupID), zap.Error(err))
		h.writeGatewayPromptTooLong(c, account, promptTooLongErr)
		return nil, false
	}
	if fallbackGroup.Platform != service.PlatformAnthropic ||
		fallbackGroup.SubscriptionType == service.SubscriptionTypeSubscription ||
		fallbackGroup.FallbackGroupIDOnInvalidRequest != nil {
		req.Ctx.Log.Warn("gateway.fallback_group_invalid",
			zap.Int64("fallback_group_id", fallbackGroup.ID),
			zap.String("fallback_platform", fallbackGroup.Platform),
			zap.String("fallback_subscription_type", fallbackGroup.SubscriptionType),
		)
		h.writeGatewayPromptTooLong(c, account, promptTooLongErr)
		return nil, false
	}

	fallbackAPIKey := cloneAPIKeyWithGroup(req.Ctx.APIKey, fallbackGroup)
	if err := h.billingCacheService.CheckBillingEligibility(
		c.Request.Context(),
		fallbackAPIKey.User,
		fallbackAPIKey,
		fallbackGroup,
		nil,
		service.PlatformFromAPIKey(fallbackAPIKey),
	); err != nil {
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.handleStreamingAwareError(c, status, code, message, req.StreamStarted)
		return nil, false
	}
	return fallbackAPIKey, true
}

func (h *GatewayHandler) writeGatewayPromptTooLong(c *gin.Context, account *service.Account, err *service.PromptTooLongError) {
	_ = h.antigravityGatewayService.WriteMappedClaudeError(c, account, err.StatusCode, err.RequestID, err.Body)
}

func (h *GatewayHandler) handleGatewayMessageSelectionError(
	c *gin.Context,
	req *gatewayMessageRequest,
	fs *FailoverState,
	err error,
	groupID *int64,
	platform string,
	fallbackUsed bool,
) bool {
	if len(fs.FailedAccountIDs) == 0 {
		markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
		req.Ctx.Log.Warn("gateway.select_account_no_available",
			zap.String("model", req.Model),
			zap.Int64p("group_id", groupID),
			zap.String("platform", platform),
			zap.Bool("fallback_used", fallbackUsed),
			zap.Error(err),
		)
		h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts: "+err.Error(), req.StreamStarted)
		return false
	}

	switch fs.HandleSelectionExhausted(c.Request.Context()) {
	case FailoverContinue:
		ctx := service.WithSingleAccountRetry(c.Request.Context(), true, h.metadataBridgeEnabled())
		c.Request = c.Request.WithContext(ctx)
		return true
	case FailoverCanceled:
		return false
	default:
		if fs.LastFailoverErr != nil {
			h.handleFailoverExhausted(c, fs.LastFailoverErr, platform, req.StreamStarted)
		} else {
			h.handleFailoverExhaustedSimple(c, 502, req.StreamStarted)
		}
		return false
	}
}

func (h *GatewayHandler) logGatewayMessageForwardError(
	c *gin.Context,
	req *gatewayMessageRequest,
	account *service.Account,
	writerSizeBeforeForward int,
	err error,
) {
	upstreamErrorAlreadyCommunicated := gatewayForwardErrorAlreadyCommunicated(c, writerSizeBeforeForward, err)
	wroteFallback := false
	if !upstreamErrorAlreadyCommunicated {
		wroteFallback = h.ensureForwardErrorResponse(c, req.StreamStarted)
	}
	forwardFailedFields := []zap.Field{
		zap.Int64("account_id", account.ID),
		zap.String("account_name", account.Name),
		zap.String("account_platform", account.Platform),
		zap.Bool("fallback_error_response_written", wroteFallback),
		zap.Bool("upstream_error_response_already_written", upstreamErrorAlreadyCommunicated),
		zap.Error(err),
	}
	if account.Proxy != nil {
		forwardFailedFields = append(forwardFailedFields,
			zap.Int64("proxy_id", account.Proxy.ID),
			zap.String("proxy_name", account.Proxy.Name),
			zap.String("proxy_host", account.Proxy.Host),
			zap.Int("proxy_port", account.Proxy.Port),
		)
	} else if account.ProxyID != nil {
		forwardFailedFields = append(forwardFailedFields, zap.Int64p("proxy_id", account.ProxyID))
	}
	req.Ctx.Log.Error("gateway.forward_failed", forwardFailedFields...)
}
