package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *GatewayHandler) handleGatewayMessageIntercept(
	c *gin.Context,
	req *gatewayMessageRequest,
	selection *service.AccountSelectionResult,
) bool {
	account := selection.Account
	if !account.IsInterceptWarmupEnabled() {
		return false
	}
	interceptType := detectInterceptType(req.Body, req.Model, req.Parsed.MaxTokens, req.IsClaudeCodeClient)
	if interceptType == InterceptTypeNone {
		return false
	}
	if selection.Acquired && selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
	if req.Stream {
		sendMockInterceptStream(c, req.Model, interceptType)
	} else {
		sendMockInterceptResponse(c, req.Model, interceptType)
	}
	return true
}

func (h *GatewayHandler) logGatewayMessageAccountSelected(
	req *gatewayMessageRequest,
	account *service.Account,
	selection *service.AccountSelectionResult,
) {
	req.Ctx.Log.Info("sticky.account_selected",
		zap.Int64("selected_account_id", account.ID),
		zap.String("account_name", account.Name),
		zap.Bool("slot_acquired", selection.Acquired),
		zap.Bool("has_wait_plan", selection.WaitPlan != nil),
		zap.Int64("sticky_bound_account_id", req.SessionBoundAccount),
		zap.Bool("sticky_honored", req.SessionBoundAccount > 0 && req.SessionBoundAccount == account.ID),
	)
}

func (h *GatewayHandler) bindGatewayMessageStickySession(
	c *gin.Context,
	req *gatewayMessageRequest,
	route *gatewayAnthropicRoute,
	account *service.Account,
) {
	if req.SessionKey == "" || (req.SessionBoundAccount != 0 && req.SessionBoundAccount != account.ID) {
		return
	}
	if err := h.gatewayService.BindStickySession(c.Request.Context(), route.APIKey.GroupID, req.SessionKey, account.ID); err != nil {
		req.Ctx.Log.Warn("gateway.bind_sticky_session_failed", zap.Int64("account_id", account.ID), zap.Error(err))
	}
}

func (h *GatewayHandler) finalizeGatewayMessageSuccess(
	c *gin.Context,
	req *gatewayMessageRequest,
	fs *FailoverState,
	account *service.Account,
	parsedReq *service.ParsedRequest,
	result *service.ForwardResult,
	apiKey *service.APIKey,
	subscription *service.UserSubscription,
	requestBody []byte,
) {
	h.incrementGatewayMessageRPM(c, req, account)
	h.fillGatewayMessageReasoningEffort(result, parsedReq)

	h.recordGatewayUsageAsync(c.Request.Context(), gatewayUsageRecordInput{
		Component:          "gateway",
		Result:             result,
		QuotaPlatform:      service.QuotaPlatform(c.Request.Context(), apiKey),
		APIKey:             apiKey,
		User:               apiKey.User,
		Account:            account,
		Subscription:       subscription,
		InboundEndpoint:    GetInboundEndpoint(c),
		UpstreamEndpoint:   GetUpstreamEndpoint(c, account.Platform),
		UserAgent:          c.GetHeader("User-Agent"),
		IPAddress:          ip.GetClientIP(c),
		RequestBody:        requestBody,
		Model:              req.Model,
		ForceCacheBilling:  fs.ForceCacheBilling,
		ChannelUsageFields: req.ChannelMapping.ToUsageFields(req.Model, result.UpstreamModel),
	}, req.Ctx.Log)
}

func (h *GatewayHandler) incrementGatewayMessageRPM(c *gin.Context, req *gatewayMessageRequest, account *service.Account) {
	if !account.IsAnthropicOAuthOrSetupToken() || account.GetBaseRPM() <= 0 {
		return
	}
	if err := h.gatewayService.IncrementAccountRPM(c.Request.Context(), account.ID); err != nil {
		req.Ctx.Log.Warn("gateway.rpm_increment_failed", zap.Int64("account_id", account.ID), zap.Error(err))
	}
}

func (h *GatewayHandler) fillGatewayMessageReasoningEffort(
	result *service.ForwardResult,
	parsedReq *service.ParsedRequest,
) {
	if result == nil || parsedReq == nil {
		return
	}
	if result.ReasoningEffort == nil {
		result.ReasoningEffort = service.NormalizeClaudeOutputEffort(parsedReq.OutputEffort)
	}
	if result.ReasoningEffort != nil || !parsedReq.ThinkingEnabled {
		return
	}
	protocolModel := result.UpstreamModel
	if protocolModel == "" {
		protocolModel = result.Model
	}
	result.ReasoningEffort = service.DefaultEffortForThinkingEnabled(protocolModel)
}
