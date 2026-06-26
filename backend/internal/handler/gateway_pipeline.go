package handler

import (
	"context"
	"net/http"
	"strconv"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type gatewayErrorWriter func(c *gin.Context, status int, errType, message string)

type gatewayRequestContext struct {
	APIKey  *service.APIKey
	Subject middleware2.AuthSubject
	Log     *zap.Logger
}

type gatewayUsageRecordInput struct {
	Component          string
	APIKey             *service.APIKey
	User               *service.User
	Account            *service.Account
	Subscription       *service.UserSubscription
	Result             *service.ForwardResult
	Model              string
	RequestBody        []byte
	QuotaPlatform      string
	ForceCacheBilling  bool
	InboundEndpoint    string
	UpstreamEndpoint   string
	UserAgent          string
	IPAddress          string
	ChannelUsageFields service.ChannelUsageFields
}

type gatewayAccountSlotOptions struct {
	GroupID          *int64
	SessionKey       string
	ReqStream        bool
	StreamStarted    *bool
	LogPrefix        string
	WriteErr         gatewayErrorWriter
	TrackWaitQueue   bool
	BindStickyOnWait bool
}

func (h *GatewayHandler) prepareGatewayRequestContext(
	c *gin.Context,
	component string,
	writeErr gatewayErrorWriter,
) (*gatewayRequestContext, bool) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		writeErr(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return nil, false
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		writeErr(c, http.StatusInternalServerError, "api_error", "User context not found")
		return nil, false
	}

	return &gatewayRequestContext{
		APIKey:  apiKey,
		Subject: subject,
		Log: requestLogger(
			c,
			component,
			zap.Int64("user_id", subject.UserID),
			zap.Int64("api_key_id", apiKey.ID),
			zap.Any("group_id", apiKey.GroupID),
		),
	}, true
}

func (h *GatewayHandler) readGatewayRequestBody(c *gin.Context, writeErr gatewayErrorWriter) ([]byte, bool) {
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			writeErr(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return nil, false
		}
		writeErr(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return nil, false
	}

	if len(body) == 0 {
		writeErr(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return nil, false
	}
	return body, true
}

func (h *GatewayHandler) bindGatewayErrorPassthrough(c *gin.Context) {
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}
}

func (h *GatewayHandler) acquireGatewayUserSlot(
	c *gin.Context,
	reqLog *zap.Logger,
	subject middleware2.AuthSubject,
	reqStream bool,
	streamStarted *bool,
	logEvent string,
) (func(), bool) {
	userReleaseFunc, err := h.concurrencyHelper.AcquireUserSlotWithWait(c, subject.UserID, subject.Concurrency, reqStream, streamStarted)
	if err != nil {
		reqLog.Warn(logEvent, zap.Error(err))
		h.handleConcurrencyError(c, err, "user", *streamStarted)
		return nil, false
	}
	return wrapReleaseOnDone(c.Request.Context(), userReleaseFunc), true
}

func (h *GatewayHandler) checkGatewayBillingEligibility(
	c *gin.Context,
	ctx context.Context,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	subscription *service.UserSubscription,
	streamStarted bool,
	writeErr gatewayErrorWriter,
	logEvent string,
) bool {
	if ctx == nil {
		ctx = c.Request.Context()
	}
	if err := h.billingCacheService.CheckBillingEligibility(ctx, apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(ctx, apiKey)); err != nil {
		reqLog.Info(logEvent, zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		if streamStarted {
			h.handleStreamingAwareError(c, status, code, message, true)
			return false
		}
		writeErr(c, status, code, message)
		return false
	}
	return true
}

func (h *GatewayHandler) acquireGatewayAccountSlot(
	c *gin.Context,
	reqLog *zap.Logger,
	selection *service.AccountSelectionResult,
	opts gatewayAccountSlotOptions,
) (func(), bool) {
	writeErr := opts.WriteErr
	if writeErr == nil {
		writeErr = h.errorResponse
	}
	if selection == nil || selection.Account == nil {
		markOpsRoutingCapacityLimited(c)
		if opts.StreamStarted != nil && *opts.StreamStarted {
			h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", true)
		} else {
			writeErr(c, http.StatusServiceUnavailable, "api_error", "No available accounts")
		}
		return nil, false
	}
	account := selection.Account
	if selection.Acquired {
		return wrapReleaseOnDone(c.Request.Context(), selection.ReleaseFunc), true
	}
	if selection.WaitPlan == nil {
		markOpsRoutingCapacityLimited(c)
		reqLog.Warn(opts.LogPrefix+".select_account_no_slot_no_wait_plan",
			zap.Int64("account_id", account.ID),
			zap.String("platform", account.Platform),
		)
		if opts.StreamStarted != nil && *opts.StreamStarted {
			h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", true)
		} else {
			writeErr(c, http.StatusServiceUnavailable, "api_error", "No available accounts")
		}
		return nil, false
	}

	accountWaitCounted := false
	if opts.TrackWaitQueue {
		canWait, err := h.concurrencyHelper.IncrementAccountWaitCount(c.Request.Context(), account.ID, selection.WaitPlan.MaxWaiting)
		if err != nil {
			reqLog.Warn(opts.LogPrefix+".account_wait_counter_increment_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		} else if !canWait {
			reqLog.Info(opts.LogPrefix+".account_wait_queue_full",
				zap.Int64("account_id", account.ID),
				zap.Int("max_waiting", selection.WaitPlan.MaxWaiting),
			)
			if opts.StreamStarted != nil && *opts.StreamStarted {
				h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending requests, please retry later", true)
			} else {
				writeErr(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending requests, please retry later")
			}
			return nil, false
		}
		if err == nil && canWait {
			accountWaitCounted = true
		}
	}
	releaseWait := func() {
		if accountWaitCounted {
			h.concurrencyHelper.DecrementAccountWaitCount(c.Request.Context(), account.ID)
			accountWaitCounted = false
		}
	}

	accountReleaseFunc, err := h.concurrencyHelper.AcquireAccountSlotWithWaitTimeout(
		c,
		account.ID,
		selection.WaitPlan.MaxConcurrency,
		selection.WaitPlan.Timeout,
		opts.ReqStream,
		opts.StreamStarted,
	)
	if err != nil {
		reqLog.Warn(opts.LogPrefix+".account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		releaseWait()
		streamStarted := false
		if opts.StreamStarted != nil {
			streamStarted = *opts.StreamStarted
		}
		h.handleConcurrencyError(c, err, "account", streamStarted)
		return nil, false
	}
	releaseWait()
	if opts.BindStickyOnWait && opts.SessionKey != "" {
		if err := h.gatewayService.BindStickySession(c.Request.Context(), opts.GroupID, opts.SessionKey, account.ID); err != nil {
			reqLog.Warn(opts.LogPrefix+".bind_sticky_session_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		}
	}
	return wrapReleaseOnDone(c.Request.Context(), accountReleaseFunc), true
}

func (h *GatewayHandler) recordGatewayUsageAsync(parent context.Context, input gatewayUsageRecordInput, reqLog *zap.Logger) {
	if input.Result == nil || input.APIKey == nil || input.Account == nil {
		return
	}
	requestPayloadHash := service.HashUsageRequestPayload(input.RequestBody)

	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.RecordUsageInput{
			Result:             input.Result,
			QuotaPlatform:      input.QuotaPlatform,
			APIKey:             input.APIKey,
			User:               input.User,
			Account:            input.Account,
			Subscription:       input.Subscription,
			InboundEndpoint:    input.InboundEndpoint,
			UpstreamEndpoint:   input.UpstreamEndpoint,
			UserAgent:          input.UserAgent,
			IPAddress:          input.IPAddress,
			RequestPayloadHash: requestPayloadHash,
			ForceCacheBilling:  input.ForceCacheBilling,
			APIKeyService:      h.apiKeyService,
			ChannelUsageFields: input.ChannelUsageFields,
		}); err != nil {
			reqLog.Error(input.Component+".record_usage_failed",
				zap.Int64("account_id", input.Account.ID),
				zap.String("model", input.Model),
				zap.Error(err),
			)
		}
	})
}

func (h *GatewayHandler) handleGatewaySelectionExhausted(
	c *gin.Context,
	fs *FailoverState,
	err error,
	reqLog *zap.Logger,
	reqModel string,
	groupID *int64,
	platform string,
	streamStarted bool,
	writeErr gatewayErrorWriter,
	handleFailoverExhausted func(*service.UpstreamFailoverError, bool),
) bool {
	if len(fs.FailedAccountIDs) == 0 {
		markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
		reqLog.Warn("gateway.select_account_no_available",
			zap.String("model", reqModel),
			zap.Int64p("group_id", groupID),
			zap.String("platform", platform),
			zap.Error(err),
		)
		writeErr(c, http.StatusServiceUnavailable, "api_error", "No available accounts: "+err.Error())
		return false
	}
	action := fs.HandleSelectionExhausted(c.Request.Context())
	switch action {
	case FailoverContinue:
		return true
	case FailoverCanceled:
		return false
	default:
		if fs.LastFailoverErr != nil {
			handleFailoverExhausted(fs.LastFailoverErr, streamStarted)
		} else {
			writeErr(c, http.StatusBadGateway, "server_error", "All available accounts exhausted")
		}
		return false
	}
}
