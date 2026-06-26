package handler

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) recordOpenAIAnthropicMessagesCyberPolicy(
	c *gin.Context,
	req *openAIAnthropicMessagesRequest,
	account *service.Account,
	forwardErrored bool,
) {
	cyberBlockKeyMsg := ""
	if service.GetOpsCyberPolicy(c) != nil {
		cyberBlockKeyMsg = service.CyberSessionBlockKey(req.APIKey.ID, c, req.Body)
	}
	h.recordCyberPolicyIfMarked(c, req.APIKey, account, req.Subscription, req.Model, forwardErrored, cyberBlockKeyMsg, req.ChannelMapping.ToUsageFields(req.Model, ""), service.HashUsageRequestPayload(req.Body))
}

func (h *OpenAIGatewayHandler) recordOpenAIAnthropicMessagesLatency(
	c *gin.Context,
	forwardStart time.Time,
	result *service.OpenAIForwardResult,
	err error,
) {
	forwardDurationMs := time.Since(forwardStart).Milliseconds()
	upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
	responseLatencyMs := forwardDurationMs
	if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
		responseLatencyMs = forwardDurationMs - upstreamLatencyMs
	}
	service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)
	if err == nil && result != nil && result.FirstTokenMs != nil {
		service.SetOpsLatencyMs(c, service.OpsTimeToFirstTokenMsKey, int64(*result.FirstTokenMs))
	}
}

func (h *OpenAIGatewayHandler) finalizeOpenAIAnthropicMessagesSuccess(
	c *gin.Context,
	req *openAIAnthropicMessagesRequest,
	state *openAIAnthropicMessagesRouteState,
	account *service.Account,
	result *service.OpenAIForwardResult,
) {
	if result != nil {
		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, result.FirstTokenMs)
	} else {
		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, nil)
	}

	h.recordOpenAIAnthropicMessagesUsage(c, req, account, result)
	req.Log.Debug("openai_messages.request_completed",
		zap.Int64("account_id", account.ID),
		zap.Int("switch_count", state.SwitchCount),
	)
}

func (h *OpenAIGatewayHandler) recordOpenAIAnthropicMessagesUsage(
	c *gin.Context,
	req *openAIAnthropicMessagesRequest,
	account *service.Account,
	result *service.OpenAIForwardResult,
) {
	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	requestPayloadHash := service.HashUsageRequestPayload(req.Body)
	inboundEndpoint := GetInboundEndpoint(c)
	upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
	cyberBlocked := service.GetOpsCyberPolicy(c) != nil

	h.submitOpenAIUsageRecordTask(c.Request.Context(), result, func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result:             result,
			APIKey:             req.APIKey,
			User:               req.APIKey.User,
			Account:            account,
			Subscription:       req.Subscription,
			InboundEndpoint:    inboundEndpoint,
			UpstreamEndpoint:   upstreamEndpoint,
			UserAgent:          userAgent,
			IPAddress:          clientIP,
			RequestPayloadHash: requestPayloadHash,
			APIKeyService:      h.apiKeyService,
			ChannelUsageFields: req.ChannelMapping.ToUsageFields(req.Model, result.UpstreamModel),
			CyberBlocked:       cyberBlocked,
		}); err != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.messages"),
				zap.Int64("user_id", req.Subject.UserID),
				zap.Int64("api_key_id", req.APIKey.ID),
				zap.Any("group_id", req.APIKey.GroupID),
				zap.String("model", req.Model),
				zap.Int64("account_id", account.ID),
			).Error("openai_messages.record_usage_failed", zap.Error(err))
		}
	})
}
