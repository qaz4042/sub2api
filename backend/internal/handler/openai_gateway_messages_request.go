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

type openAIAnthropicMessagesRequest struct {
	APIKey          *service.APIKey
	Subject         middleware2.AuthSubject
	Log             *zap.Logger
	Body            []byte
	Model           string
	RoutingModel    string
	PreferredMapped string
	EffectiveMapped string
	Stream          bool
	ChannelMapping  service.ChannelMappingResult
	MappedBody      func(bool, string) []byte
	Subscription    *service.UserSubscription
	SessionHash     string
	PromptCacheKey  string
	RoutingStart    time.Time
	UserRelease     func()
}

func (r *openAIAnthropicMessagesRequest) release() {
	if r == nil || r.UserRelease == nil {
		return
	}
	r.UserRelease()
	r.UserRelease = nil
}

func (h *OpenAIGatewayHandler) prepareOpenAIAnthropicMessagesRequest(
	c *gin.Context,
	streamStarted *bool,
	requestStart time.Time,
) (*openAIAnthropicMessagesRequest, bool) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.anthropicErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return nil, false
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.anthropicErrorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return nil, false
	}
	reqLog := requestLogger(
		c,
		"handler.openai_gateway.messages",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	if apiKey.Group != nil && !apiKey.Group.AllowMessagesDispatch {
		h.anthropicErrorResponse(c, http.StatusForbidden, "permission_error", "This group does not allow /v1/messages dispatch")
		return nil, false
	}
	if !h.ensureResponsesDependencies(c, reqLog) {
		return nil, false
	}

	body, ok := h.readOpenAIAnthropicMessagesBody(c)
	if !ok {
		return nil, false
	}
	reqModel, reqStream, reqLog, ok := h.validateOpenAIAnthropicMessagesBody(c, body, reqLog)
	if !ok {
		return nil, false
	}

	req := &openAIAnthropicMessagesRequest{
		APIKey:          apiKey,
		Subject:         subject,
		Log:             reqLog,
		Body:            body,
		Model:           reqModel,
		RoutingModel:    service.NormalizeOpenAICompatRequestedModel(reqModel),
		PreferredMapped: resolveOpenAIMessagesDispatchMappedModel(apiKey, reqModel),
		Stream:          reqStream,
	}
	req.EffectiveMapped = req.PreferredMapped
	if !h.applyOpenAIAnthropicMessagesPolicies(c, req) {
		return nil, false
	}
	if !h.acquireOpenAIAnthropicMessagesResources(c, req, streamStarted, requestStart) {
		req.release()
		return nil, false
	}

	req.SessionHash = h.gatewayService.GenerateSessionHash(c, body)
	req.PromptCacheKey = h.gatewayService.ExtractSessionID(c, body)
	req.SessionHash, req.PromptCacheKey = resolveOpenAIMessagesMetadataSession(req.SessionHash, req.PromptCacheKey, req.Model, body)
	if h.rejectIfCyberSessionBlocked(c, apiKey, body, reqModel, cyberBlockFormatAnthropic) {
		req.release()
		return nil, false
	}
	return req, true
}

func (h *OpenAIGatewayHandler) readOpenAIAnthropicMessagesBody(c *gin.Context) ([]byte, bool) {
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.anthropicErrorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return nil, false
		}
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return nil, false
	}
	if len(body) == 0 {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return nil, false
	}
	return body, true
}

func (h *OpenAIGatewayHandler) validateOpenAIAnthropicMessagesBody(
	c *gin.Context,
	body []byte,
	reqLog *zap.Logger,
) (string, bool, *zap.Logger, bool) {
	if !gjson.ValidBytes(body) {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return "", false, reqLog, false
	}

	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return "", false, reqLog, false
	}
	reqModel := modelResult.String()
	reqStream := gjson.GetBytes(body, "stream").Bool()
	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))
	return reqModel, reqStream, reqLog, true
}

func (h *OpenAIGatewayHandler) applyOpenAIAnthropicMessagesPolicies(c *gin.Context, req *openAIAnthropicMessagesRequest) bool {
	setOpsRequestContext(c, req.Model, req.Stream)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(req.Stream, false)))

	if decision := h.checkContentModeration(c, req.Log, req.APIKey, req.Subject, service.ContentModerationProtocolAnthropicMessages, req.Model, req.Body); decision != nil && decision.Blocked {
		h.anthropicErrorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
		return false
	}

	req.ChannelMapping, _ = h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), req.APIKey.GroupID, req.Model)
	req.MappedBody = newOpenAIModelMappedBodyCache(req.Body, h.gatewayService.ReplaceModelInBody)

	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}
	return true
}

func (h *OpenAIGatewayHandler) acquireOpenAIAnthropicMessagesResources(
	c *gin.Context,
	req *openAIAnthropicMessagesRequest,
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
		req.Log.Info("openai_messages.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.anthropicStreamingAwareError(c, status, code, message, streamStartedValue(streamStarted))
		return false
	}
	return true
}

func resolveOpenAIMessagesMetadataSession(sessionHash, promptCacheKey, reqModel string, body []byte) (string, string) {
	// Anthropic metadata.user_id 只作为账号粘性信号。上游 GPT/Codex 缓存键
	// 交给 ForwardAsAnthropic 从 cache_control 或完整消息 digest 派生，避免
	// 固定 metadata key 压住后续 turn 的缓存滚动。
	if sessionHash != "" {
		return sessionHash, promptCacheKey
	}
	if userID := strings.TrimSpace(gjson.GetBytes(body, "metadata.user_id").String()); userID != "" {
		seed := reqModel + "-" + userID
		sessionHash = service.DeriveSessionHashFromSeed(seed)
	}
	return sessionHash, promptCacheKey
}
