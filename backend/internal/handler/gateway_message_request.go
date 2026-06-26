package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *GatewayHandler) prepareGatewayMessageRequest(c *gin.Context, reqCtx *gatewayRequestContext) (*gatewayMessageRequest, bool) {
	setOpsRequestContext(c, "", false)

	body, ok := h.readGatewayRequestBody(c, h.errorResponse)
	if !ok {
		return nil, false
	}

	parsedReq, ok := h.parseGatewayMessageBody(c, body)
	if !ok {
		return nil, false
	}
	req := &gatewayMessageRequest{
		Ctx:    reqCtx,
		Body:   body,
		Parsed: parsedReq,
		Model:  parsedReq.Model,
		Stream: parsedReq.Stream,
	}
	reqCtx.Log = reqCtx.Log.With(zap.String("model", req.Model), zap.Bool("stream", req.Stream))

	req.ChannelMapping, _ = h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), reqCtx.APIKey.GroupID, req.Model)
	if !h.applyGatewayMessageRequestContext(c, req) {
		return nil, false
	}
	if !h.validateGatewayMessageRequest(c, req) {
		return nil, false
	}
	h.bindGatewayErrorPassthrough(c)

	req.Subscription, _ = middleware2.GetSubscriptionFromContext(c)
	userRelease, ok := h.acquireGatewayUserSlot(c, reqCtx.Log, reqCtx.Subject, req.Stream, &req.StreamStarted, "gateway.user_slot_acquire_failed")
	if !ok {
		return nil, false
	}
	req.ReleaseUserSlot = userRelease

	if !h.checkGatewayBillingEligibility(
		c,
		c.Request.Context(),
		reqCtx.Log,
		reqCtx.APIKey,
		req.Subscription,
		req.StreamStarted,
		h.errorResponse,
		"gateway.billing_eligibility_check_failed",
	) {
		req.ReleaseUserSlot()
		req.ReleaseUserSlot = nil
		return nil, false
	}

	h.initializeGatewayMessageSession(c, req)
	return req, true
}

func (h *GatewayHandler) parseGatewayMessageBody(c *gin.Context, body []byte) (*service.ParsedRequest, bool) {
	bodyRef := service.NewRequestBodyRef(body)
	parsedReq, err := service.ParseGatewayRequest(bodyRef, domain.PlatformAnthropic)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return nil, false
	}
	return parsedReq, true
}

func (h *GatewayHandler) applyGatewayMessageRequestContext(c *gin.Context, req *gatewayMessageRequest) bool {
	if isMaxTokensOneHaikuRequest(req.Model, req.Parsed.MaxTokens) {
		ctx := service.WithIsMaxTokensOneHaikuRequest(c.Request.Context(), true, h.metadataBridgeEnabled())
		c.Request = c.Request.WithContext(ctx)
	}

	SetClaudeCodeClientContext(c, req.Body, req.Parsed)
	req.IsClaudeCodeClient = service.IsClaudeCodeClient(c.Request.Context())
	if !h.checkClaudeCodeVersion(c) {
		return false
	}

	ctx := service.WithThinkingEnabled(c.Request.Context(), req.Parsed.ThinkingEnabled, h.metadataBridgeEnabled())
	c.Request = c.Request.WithContext(ctx)

	setOpsRequestContext(c, req.Model, req.Stream)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(req.Stream, false)))
	return true
}

func (h *GatewayHandler) validateGatewayMessageRequest(c *gin.Context, req *gatewayMessageRequest) bool {
	if req.Model == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return false
	}

	decision := h.checkContentModeration(
		c,
		req.Ctx.Log,
		req.Ctx.APIKey,
		req.Ctx.Subject,
		service.ContentModerationProtocolAnthropicMessages,
		req.Model,
		req.Body,
	)
	if decision != nil && decision.Blocked {
		h.errorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
		return false
	}
	return true
}

func (h *GatewayHandler) initializeGatewayMessageSession(c *gin.Context, req *gatewayMessageRequest) {
	req.Parsed.GroupID = req.Ctx.APIKey.GroupID
	req.Parsed.SessionContext = &service.SessionContext{
		ClientIP:  ip.GetClientIP(c),
		UserAgent: c.GetHeader("User-Agent"),
		APIKeyID:  req.Ctx.APIKey.ID,
	}

	req.SessionHash = h.gatewayService.GenerateSessionHash(req.Parsed)
	req.Ctx.Log.Info("sticky.session_hash_generated",
		zap.String("session_hash", req.SessionHash),
		zap.String("metadata_user_id_raw", req.Parsed.MetadataUserID),
	)

	req.Platform = gatewayMessagePlatform(c, req.Ctx.APIKey)
	req.SessionKey = req.SessionHash
	if req.Platform == service.PlatformGemini && req.SessionHash != "" {
		req.SessionKey = "gemini:" + req.SessionHash
	}

	if req.SessionKey == "" {
		req.Ctx.Log.Info("sticky.no_session_key", zap.String("session_hash", req.SessionHash))
		return
	}

	req.SessionBoundAccount, _ = h.gatewayService.GetCachedSessionAccountID(c.Request.Context(), req.Ctx.APIKey.GroupID, req.SessionKey)
	req.Ctx.Log.Info("sticky.cache_lookup",
		zap.String("session_key", req.SessionKey),
		zap.Int64("bound_account_id", req.SessionBoundAccount),
	)
	if req.SessionBoundAccount > 0 {
		prefetchedGroupID := int64(0)
		if req.Ctx.APIKey.GroupID != nil {
			prefetchedGroupID = *req.Ctx.APIKey.GroupID
		}
		ctx := service.WithPrefetchedStickySession(c.Request.Context(), req.SessionBoundAccount, prefetchedGroupID, h.metadataBridgeEnabled())
		c.Request = c.Request.WithContext(ctx)
	}
	req.HasBoundSession = req.SessionBoundAccount > 0
}

func gatewayMessagePlatform(c *gin.Context, apiKey *service.APIKey) string {
	if forcePlatform, ok := middleware2.GetForcePlatformFromContext(c); ok {
		return forcePlatform
	}
	if apiKey != nil && apiKey.Group != nil {
		return apiKey.Group.Platform
	}
	return ""
}
