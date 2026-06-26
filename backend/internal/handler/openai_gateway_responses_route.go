package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) runOpenAIResponsesRoute(
	c *gin.Context,
	req *openAIResponsesRequest,
	streamStarted *bool,
) {
	state := &openAIResponsesRouteState{
		MaxAccountSwitches:    h.maxAccountSwitches,
		FailedAccountIDs:      make(map[int64]struct{}),
		SameAccountRetryCount: make(map[int64]int),
	}

	for {
		selection, ok := h.selectOpenAIResponsesAccount(c, req, state, streamStarted)
		if !ok {
			return
		}

		account := selection.Account
		req.SessionHash = ensureOpenAIPoolModeSessionHash(req.SessionHash, account)
		req.Log.Debug("openai.account_selected", zap.Int64("account_id", account.ID), zap.String("account_name", account.Name))
		setOpsSelectedAccount(c, account.ID, account.Platform)

		accountRelease, acquired := h.acquireResponsesAccountSlot(c, req.APIKey.GroupID, req.SessionHash, selection, req.Stream, streamStarted, req.Log)
		if !acquired {
			return
		}

		outcome := h.forwardOpenAIResponsesAttempt(c, req, state, account, accountRelease, streamStarted)
		if outcome == gatewayAttemptContinue {
			continue
		}
		return
	}
}

type openAIResponsesRouteState struct {
	SwitchCount           int
	MaxAccountSwitches    int
	FailedAccountIDs      map[int64]struct{}
	SameAccountRetryCount map[int64]int
	LastFailoverErr       *service.UpstreamFailoverError
}
