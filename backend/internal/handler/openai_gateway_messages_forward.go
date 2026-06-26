package handler

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) forwardOpenAIAnthropicMessagesAttempt(
	c *gin.Context,
	req *openAIAnthropicMessagesRequest,
	state *openAIAnthropicMessagesRouteState,
	account *service.Account,
	accountRelease func(),
	streamStarted *bool,
) gatewayAttemptOutcome {
	service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(req.RoutingStart).Milliseconds())
	forwardStart := time.Now()

	defaultMappedModel := strings.TrimSpace(req.EffectiveMapped)
	forwardBody := req.MappedBody(req.ChannelMapping.Mapped, req.ChannelMapping.MappedModel)
	writerSizeBeforeForward := c.Writer.Size()
	result, err := func() (*service.OpenAIForwardResult, error) {
		defer func() {
			if accountRelease != nil {
				accountRelease()
			}
		}()
		return h.gatewayService.ForwardAsAnthropic(c.Request.Context(), c, account, forwardBody, req.PromptCacheKey, defaultMappedModel)
	}()

	h.recordOpenAIAnthropicMessagesCyberPolicy(c, req, account, err != nil)
	h.recordOpenAIAnthropicMessagesLatency(c, forwardStart, result, err)
	if err != nil {
		return h.handleOpenAIAnthropicMessagesForwardError(c, req, state, account, writerSizeBeforeForward, result, err, streamStarted)
	}

	h.finalizeOpenAIAnthropicMessagesSuccess(c, req, state, account, result)
	return gatewayAttemptDone
}

func (h *OpenAIGatewayHandler) retryOpenAIAnthropicMessagesSameAccount(
	c *gin.Context,
	req *openAIAnthropicMessagesRequest,
	state *openAIAnthropicMessagesRouteState,
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
	req.Log.Warn("openai_messages.pool_mode_same_account_retry",
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
