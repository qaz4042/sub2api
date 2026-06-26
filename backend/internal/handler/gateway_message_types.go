package handler

import "github.com/Wei-Shaw/sub2api/internal/service"

type gatewayMessageRequest struct {
	Ctx                 *gatewayRequestContext
	Body                []byte
	Parsed              *service.ParsedRequest
	Model               string
	Stream              bool
	ChannelMapping      service.ChannelMappingResult
	Subscription        *service.UserSubscription
	StreamStarted       bool
	IsClaudeCodeClient  bool
	Platform            string
	SessionHash         string
	SessionKey          string
	SessionBoundAccount int64
	HasBoundSession     bool
	ReleaseUserSlot     func()
}

type gatewayAnthropicRoute struct {
	APIKey          *service.APIKey
	Subscription    *service.UserSubscription
	FallbackGroupID *int64
	FallbackUsed    bool
}

type gatewayMessageAttempt struct {
	Account                 *service.Account
	Selection               *service.AccountSelectionResult
	Parsed                  *service.ParsedRequest
	Body                    []byte
	AccountRelease          func()
	QueueRelease            func()
	WriterSizeBeforeForward int
	Result                  *service.ForwardResult
}

type gatewayAttemptOutcome int

const (
	gatewayAttemptDone gatewayAttemptOutcome = iota
	gatewayAttemptContinue
	gatewayAttemptRetryFallback
)

type gatewaySelectionOutcome int

const (
	gatewaySelectionSelected gatewaySelectionOutcome = iota
	gatewaySelectionRetry
	gatewaySelectionDone
)

func (a *gatewayMessageAttempt) release() {
	if a == nil {
		return
	}
	if a.QueueRelease != nil {
		a.QueueRelease()
		a.QueueRelease = nil
	}
	if a.Parsed != nil {
		a.Parsed.OnUpstreamAccepted = nil
	}
	if a.AccountRelease != nil {
		a.AccountRelease()
		a.AccountRelease = nil
	}
}
