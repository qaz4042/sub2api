package handler

import (
	"context"
	"errors"
	"strings"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const openAIWSFirstMessageReadTimeout = 30 * time.Second

type openAIWSFirstMessage struct {
	Payload                []byte
	Model                  string
	PreviousResponseID     string
	PreviousResponseIDKind string
}

type openAIWSFirstMessageParseError struct {
	Status coderws.StatusCode
	Reason string
}

func readOpenAIWSFirstMessage(ctx context.Context, conn *coderws.Conn, reqLog *zap.Logger, clientIP string) (*openAIWSFirstMessage, bool) {
	readCtx, cancel := context.WithTimeout(ctx, openAIWSFirstMessageReadTimeout)
	msgType, payload, err := conn.Read(readCtx)
	cancel()
	if err != nil {
		closeStatus, closeReason := summarizeWSCloseErrorForLog(err)
		reqLog.Warn("openai.websocket_read_first_message_failed",
			zap.Error(err),
			zap.String("client_ip", clientIP),
			zap.String("close_status", closeStatus),
			zap.String("close_reason", closeReason),
			zap.Duration("read_timeout", openAIWSFirstMessageReadTimeout),
		)
		closeOpenAIClientWS(conn, coderws.StatusPolicyViolation, "missing first response.create message")
		return nil, false
	}
	if msgType != coderws.MessageText && msgType != coderws.MessageBinary {
		closeOpenAIClientWS(conn, coderws.StatusPolicyViolation, "unsupported websocket message type")
		return nil, false
	}
	first, parseErr := parseOpenAIWSFirstMessagePayload(payload)
	if parseErr != nil {
		closeOpenAIClientWS(conn, parseErr.Status, parseErr.Reason)
		return nil, false
	}
	return first, true
}

func parseOpenAIWSFirstMessagePayload(payload []byte) (*openAIWSFirstMessage, *openAIWSFirstMessageParseError) {
	if !gjson.ValidBytes(payload) {
		return nil, &openAIWSFirstMessageParseError{Status: coderws.StatusPolicyViolation, Reason: "invalid JSON payload"}
	}

	reqModel := strings.TrimSpace(gjson.GetBytes(payload, "model").String())
	if reqModel == "" {
		return nil, &openAIWSFirstMessageParseError{Status: coderws.StatusPolicyViolation, Reason: "model is required in first response.create payload"}
	}
	previousResponseID := strings.TrimSpace(gjson.GetBytes(payload, "previous_response_id").String())
	previousResponseIDKind := service.ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
	if previousResponseID != "" && previousResponseIDKind == service.OpenAIPreviousResponseIDKindMessageID {
		return nil, &openAIWSFirstMessageParseError{Status: coderws.StatusPolicyViolation, Reason: "previous_response_id must be a response.id (resp_*), not a message id"}
	}

	return &openAIWSFirstMessage{
		Payload:                payload,
		Model:                  reqModel,
		PreviousResponseID:     previousResponseID,
		PreviousResponseIDKind: previousResponseIDKind,
	}, nil
}

type openAIWSTurnSlots struct {
	h          *OpenAIGatewayHandler
	ctx        context.Context
	conn       *coderws.Conn
	userID     int64
	userLimit  int
	user       func()
	account    func()
	accountID  int64
	accountMax int
}

func newOpenAIWSTurnSlots(h *OpenAIGatewayHandler, ctx context.Context, conn *coderws.Conn, subject middleware2.AuthSubject) *openAIWSTurnSlots {
	return &openAIWSTurnSlots{
		h:         h,
		ctx:       ctx,
		conn:      conn,
		userID:    subject.UserID,
		userLimit: subject.Concurrency,
	}
}

func (s *openAIWSTurnSlots) releaseAccount() {
	if s.account != nil {
		s.account()
		s.account = nil
	}
}

func (s *openAIWSTurnSlots) releaseAll() {
	s.releaseAccount()
	if s.user != nil {
		s.user()
		s.user = nil
	}
}

func (s *openAIWSTurnSlots) acquireInitialUser(reqLog *zap.Logger) bool {
	release, acquired, err := s.h.concurrencyHelper.TryAcquireUserSlot(s.ctx, s.userID, s.userLimit)
	if err != nil {
		reqLog.Warn("openai.websocket_user_slot_acquire_failed", zap.Error(err))
		closeOpenAIClientWS(s.conn, coderws.StatusInternalError, "failed to acquire user concurrency slot")
		return false
	}
	if !acquired {
		closeOpenAIClientWS(s.conn, coderws.StatusTryAgainLater, "too many concurrent requests, please retry later")
		return false
	}
	s.user = wrapReleaseOnDone(s.ctx, release)
	return true
}

func (s *openAIWSTurnSlots) ensureUserHeld(reqLog *zap.Logger) bool {
	if s.user != nil {
		return true
	}
	release, acquired, err := s.h.concurrencyHelper.TryAcquireUserSlot(s.ctx, s.userID, s.userLimit)
	if err != nil {
		reqLog.Warn("openai.websocket_user_slot_reacquire_failed", zap.Error(err))
		closeOpenAIClientWS(s.conn, coderws.StatusInternalError, "failed to acquire user concurrency slot")
		return false
	}
	if !acquired {
		closeOpenAIClientWS(s.conn, coderws.StatusTryAgainLater, "too many concurrent requests, please retry later")
		return false
	}
	s.user = wrapReleaseOnDone(s.ctx, release)
	return true
}

func (s *openAIWSTurnSlots) bindAccountSelection(selection *service.AccountSelectionResult, reqLog *zap.Logger) bool {
	account := selection.Account
	s.accountID = account.ID
	s.accountMax = account.Concurrency
	if selection.WaitPlan != nil && selection.WaitPlan.MaxConcurrency > 0 {
		s.accountMax = selection.WaitPlan.MaxConcurrency
	}

	accountRelease := selection.ReleaseFunc
	if !selection.Acquired {
		if selection.WaitPlan == nil {
			closeOpenAIClientWS(s.conn, coderws.StatusTryAgainLater, "account is busy, please retry later")
			return false
		}
		fastRelease, fastAcquired, err := s.h.concurrencyHelper.TryAcquireAccountSlot(s.ctx, account.ID, selection.WaitPlan.MaxConcurrency)
		if err != nil {
			reqLog.Warn("openai.websocket_account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
			closeOpenAIClientWS(s.conn, coderws.StatusInternalError, "failed to acquire account concurrency slot")
			return false
		}
		if !fastAcquired {
			closeOpenAIClientWS(s.conn, coderws.StatusTryAgainLater, "account is busy, please retry later")
			return false
		}
		accountRelease = fastRelease
	}
	s.account = wrapReleaseOnDone(s.ctx, accountRelease)
	return true
}

func (s *openAIWSTurnSlots) acquireNextTurn() error {
	s.releaseAll()

	userRelease, userAcquired, err := s.h.concurrencyHelper.TryAcquireUserSlot(s.ctx, s.userID, s.userLimit)
	if err != nil {
		return service.NewOpenAIWSClientCloseError(coderws.StatusInternalError, "failed to acquire user concurrency slot", err)
	}
	if !userAcquired {
		return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "too many concurrent requests, please retry later", nil)
	}
	accountRelease, accountAcquired, err := s.h.concurrencyHelper.TryAcquireAccountSlot(s.ctx, s.accountID, s.accountMax)
	if err != nil {
		if userRelease != nil {
			userRelease()
		}
		return service.NewOpenAIWSClientCloseError(coderws.StatusInternalError, "failed to acquire account concurrency slot", err)
	}
	if !accountAcquired {
		if userRelease != nil {
			userRelease()
		}
		return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "account is busy, please retry later", nil)
	}
	s.user = wrapReleaseOnDone(s.ctx, userRelease)
	s.account = wrapReleaseOnDone(s.ctx, accountRelease)
	return nil
}

type openAIWSIngressHooksInput struct {
	Context             context.Context
	GinContext          *gin.Context
	Conn                *coderws.Conn
	RequestLog          *zap.Logger
	APIKey              *service.APIKey
	Subject             middleware2.AuthSubject
	Account             *service.Account
	Subscription        *service.UserSubscription
	RequestModel        string
	ClientIP            string
	UserAgent           string
	CyberBlockKey       string
	ChannelMapping      service.ChannelMappingResult
	TurnSlots           *openAIWSTurnSlots
	RequestPayloadHash  *string
	CyberBlockedConnPtr *bool
}

func (h *OpenAIGatewayHandler) buildOpenAIWSIngressHooks(in openAIWSIngressHooksInput) *service.OpenAIWSIngressHooks {
	return &service.OpenAIWSIngressHooks{
		InitialRequestModel: in.RequestModel,
		BeforeRequest: func(turn int, payload []byte, originalModel string) error {
			if turn == 1 {
				return nil
			}
			if !gjson.ValidBytes(payload) {
				return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "invalid websocket request payload", errors.New("invalid json"))
			}
			model := strings.TrimSpace(originalModel)
			if model == "" {
				model = strings.TrimSpace(gjson.GetBytes(payload, "model").String())
			}
			if model == "" {
				model = in.RequestModel
			}
			if decision := h.checkContentModeration(in.GinContext, in.RequestLog, in.APIKey, in.Subject, service.ContentModerationProtocolOpenAIResponses, model, payload); decision != nil && decision.Blocked {
				writeContentModerationWSError(in.Context, in.Conn, decision)
				return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, decision.Message, nil)
			}
			return nil
		},
		BeforeTurn: func(turn int) error {
			// turn==1 的会话屏蔽已由握手层检查覆盖；连接内 flag 只拦截后续 turn。
			if in.CyberBlockedConnPtr != nil && *in.CyberBlockedConnPtr {
				return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, cyberSessionBlockedClientMsg, nil)
			}
			if turn == 1 {
				return nil
			}
			return in.TurnSlots.acquireNextTurn()
		},
		AfterTurn: func(turn int, result *service.OpenAIForwardResult, turnErr error) {
			defer clearCyberPolicyTurnState(in.GinContext)
			in.TurnSlots.releaseAll()
			h.recordCyberPolicyIfMarked(
				in.GinContext,
				in.APIKey,
				in.Account,
				in.Subscription,
				in.RequestModel,
				turnErr != nil,
				in.CyberBlockKey,
				in.ChannelMapping.ToUsageFields(in.RequestModel, ""),
				*in.RequestPayloadHash,
			)
			if service.GetOpsCyberPolicy(in.GinContext) != nil && in.CyberBlockedConnPtr != nil {
				*in.CyberBlockedConnPtr = true
			}
			if turnErr != nil {
				if result == nil || result.ImageCount <= 0 {
					return
				}
				if service.GetOpsCyberPolicy(in.GinContext) != nil {
					return
				}
				in.RequestLog.Warn("openai.websocket_partial_error_with_image_result",
					zap.Int64("account_id", in.Account.ID),
					zap.Int("image_count", result.ImageCount),
					zap.Error(turnErr),
				)
			}
			if result == nil {
				return
			}
			if in.Account.Type == service.AccountTypeOAuth {
				h.gatewayService.UpdateCodexUsageSnapshotFromHeaders(in.Context, in.Account.ID, result.ResponseHeaders)
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(in.Account.ID, true, result.FirstTokenMs)
			inboundEndpoint := GetInboundEndpoint(in.GinContext)
			upstreamEndpoint := GetUpstreamEndpoint(in.GinContext, in.Account.Platform)
			cyberBlocked := service.GetOpsCyberPolicy(in.GinContext) != nil
			h.submitOpenAIUsageRecordTask(in.Context, result, func(taskCtx context.Context) {
				if err := h.gatewayService.RecordUsage(taskCtx, &service.OpenAIRecordUsageInput{
					Result:             result,
					APIKey:             in.APIKey,
					User:               in.APIKey.User,
					Account:            in.Account,
					Subscription:       in.Subscription,
					InboundEndpoint:    inboundEndpoint,
					UpstreamEndpoint:   upstreamEndpoint,
					UserAgent:          in.UserAgent,
					IPAddress:          in.ClientIP,
					RequestPayloadHash: *in.RequestPayloadHash,
					APIKeyService:      h.apiKeyService,
					ChannelUsageFields: in.ChannelMapping.ToUsageFields(in.RequestModel, result.UpstreamModel),
					CyberBlocked:       cyberBlocked,
				}); err != nil {
					in.RequestLog.Error("openai.websocket_record_usage_failed",
						zap.Int64("account_id", in.Account.ID),
						zap.String("request_id", result.RequestID),
						zap.Error(err),
					)
				}
			})
		},
	}
}
