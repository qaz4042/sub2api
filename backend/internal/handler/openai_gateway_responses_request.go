package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

type openAIResponsesRequest struct {
	APIKey             *service.APIKey
	Subject            middleware2.AuthSubject
	Log                *zap.Logger
	Body               []byte
	SessionHashBody    []byte
	Model              string
	Stream             bool
	PreviousResponseID string
	ChannelMapping     service.ChannelMappingResult
	ForwardBody        []byte
	Subscription       *service.UserSubscription
	SessionHash        string
	RequireCompact     bool
	RoutingStart       time.Time
	UserRelease        func()
	ImageRelease       func()
}

func (r *openAIResponsesRequest) release() {
	if r == nil {
		return
	}
	if r.UserRelease != nil {
		r.UserRelease()
		r.UserRelease = nil
	}
	if r.ImageRelease != nil {
		r.ImageRelease()
		r.ImageRelease = nil
	}
}

func (h *OpenAIGatewayHandler) prepareOpenAIResponsesRequest(
	c *gin.Context,
	streamStarted *bool,
	requestStart time.Time,
) (*openAIResponsesRequest, bool) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return nil, false
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return nil, false
	}

	reqLog := requestLogger(
		c,
		"handler.openai_gateway.responses",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return nil, false
	}

	body, sessionHashBody, ok := h.readAndNormalizeOpenAIResponsesBody(c)
	if !ok {
		return nil, false
	}

	reqModel, reqStream, previousResponseID, reqLog, ok := h.validateOpenAIResponsesBody(c, body, reqLog)
	if !ok {
		return nil, false
	}

	req := &openAIResponsesRequest{
		APIKey:             apiKey,
		Subject:            subject,
		Log:                reqLog,
		Body:               body,
		SessionHashBody:    sessionHashBody,
		Model:              reqModel,
		Stream:             reqStream,
		PreviousResponseID: previousResponseID,
	}
	if !h.applyOpenAIResponsesRequestPolicies(c, req, streamStarted) {
		return nil, false
	}

	if !h.acquireOpenAIResponsesPreForwardResources(c, req, streamStarted, requestStart) {
		req.release()
		return nil, false
	}

	req.SessionHash = h.gatewayService.GenerateSessionHash(c, req.SessionHashBody)
	if h.rejectIfCyberSessionBlocked(c, apiKey, req.SessionHashBody, reqModel, cyberBlockFormatResponses) {
		req.release()
		return nil, false
	}
	req.RequireCompact = isOpenAIRemoteCompactPath(c)
	return req, true
}

func (h *OpenAIGatewayHandler) readAndNormalizeOpenAIResponsesBody(c *gin.Context) ([]byte, []byte, bool) {
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return nil, nil, false
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return nil, nil, false
	}
	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return nil, nil, false
	}

	setOpsRequestContext(c, "", false)
	sessionHashBody := body
	if service.IsOpenAIResponsesCompactPathForTest(c) {
		if compactSeed := strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String()); compactSeed != "" {
			c.Set(service.OpenAICompactSessionSeedKeyForTest(), compactSeed)
		}
		normalizedCompactBody, normalizedCompact, compactErr := service.NormalizeOpenAICompactRequestBodyForTest(body)
		if compactErr != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to normalize compact request body")
			return nil, nil, false
		}
		if normalizedCompact {
			body = normalizedCompactBody
		}
	}
	return body, sessionHashBody, true
}

func (h *OpenAIGatewayHandler) validateOpenAIResponsesBody(
	c *gin.Context,
	body []byte,
	reqLog *zap.Logger,
) (string, bool, string, *zap.Logger, bool) {
	if !gjson.ValidBytes(body) {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return "", false, "", reqLog, false
	}

	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return "", false, "", reqLog, false
	}
	reqModel := modelResult.String()

	reqStream, ok := parseOpenAICompatibleStream(body)
	if !ok {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", invalidStreamFieldTypeMessage)
		return "", false, "", reqLog, false
	}

	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))
	previousResponseID := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
	if previousResponseID == "" {
		return reqModel, reqStream, "", reqLog, true
	}

	previousResponseIDKind := service.ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
	reqLog = reqLog.With(
		zap.Bool("has_previous_response_id", true),
		zap.String("previous_response_id_kind", previousResponseIDKind),
		zap.Int("previous_response_id_len", len(previousResponseID)),
	)
	if previousResponseIDKind == service.OpenAIPreviousResponseIDKindMessageID {
		reqLog.Warn("openai.request_validation_failed",
			zap.String("reason", "previous_response_id_looks_like_message_id"),
		)
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "previous_response_id must be a response.id (resp_*), not a message id")
		return "", false, "", reqLog, false
	}
	reqLog.Warn("openai.request_validation_failed",
		zap.String("reason", "previous_response_id_requires_wsv2"),
	)
	h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "previous_response_id is only supported on Responses WebSocket v2")
	return "", false, "", reqLog, false
}

func (h *OpenAIGatewayHandler) applyOpenAIResponsesRequestPolicies(c *gin.Context, req *openAIResponsesRequest, streamStarted *bool) bool {
	setOpsRequestContext(c, req.Model, req.Stream)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(req.Stream, false)))

	if decision := h.checkContentModeration(c, req.Log, req.APIKey, req.Subject, service.ContentModerationProtocolOpenAIResponses, req.Model, req.Body); decision != nil && decision.Blocked {
		h.errorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
		return false
	}

	imageIntent := service.IsImageGenerationIntent("/v1/responses", req.Model, req.Body)
	if imageIntent && !service.GroupAllowsImageGeneration(req.APIKey.Group) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return false
	}
	if imageIntent {
		started := streamStarted != nil && *streamStarted
		imageRelease, imageAcquired := h.acquireImageGenerationSlot(c, started)
		if !imageAcquired {
			return false
		}
		req.ImageRelease = imageRelease
	}

	req.ChannelMapping, _ = h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), req.APIKey.GroupID, req.Model)
	req.ForwardBody = openAIModelMappedBody(req.Body, req.ChannelMapping.Mapped, req.ChannelMapping.MappedModel, h.gatewayService.ReplaceModelInBody)

	if !h.validateFunctionCallOutputRequest(c, req.Body, req.Log) {
		return false
	}

	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}
	return true
}

func (h *OpenAIGatewayHandler) acquireOpenAIResponsesPreForwardResources(
	c *gin.Context,
	req *openAIResponsesRequest,
	streamStarted *bool,
	requestStart time.Time,
) bool {
	req.Subscription, _ = middleware2.GetSubscriptionFromContext(c)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	req.RoutingStart = time.Now()

	userRelease, acquired := h.acquireResponsesUserSlot(c, req.Subject.UserID, req.Subject.Concurrency, req.Stream, streamStarted, req.Log)
	if !acquired {
		return false
	}
	req.UserRelease = userRelease

	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), req.APIKey.User, req.APIKey, req.APIKey.Group, req.Subscription, service.QuotaPlatform(c.Request.Context(), req.APIKey)); err != nil {
		req.Log.Info("openai.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		started := streamStarted != nil && *streamStarted
		h.handleStreamingAwareError(c, status, code, message, started)
		return false
	}
	return true
}
