package handler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func TestParseOpenAIWSFirstMessagePayload(t *testing.T) {
	t.Run("valid with response previous id", func(t *testing.T) {
		first, parseErr := parseOpenAIWSFirstMessagePayload([]byte(`{"model":" gpt-5.1 ","previous_response_id":"resp_123"}`))

		require.Nil(t, parseErr)
		require.Equal(t, "gpt-5.1", first.Model)
		require.Equal(t, "resp_123", first.PreviousResponseID)
		require.Equal(t, service.OpenAIPreviousResponseIDKindResponseID, first.PreviousResponseIDKind)
	})

	t.Run("invalid json", func(t *testing.T) {
		first, parseErr := parseOpenAIWSFirstMessagePayload([]byte(`{`))

		require.Nil(t, first)
		require.NotNil(t, parseErr)
		require.Equal(t, coderws.StatusPolicyViolation, parseErr.Status)
		require.Equal(t, "invalid JSON payload", parseErr.Reason)
	})

	t.Run("missing model", func(t *testing.T) {
		first, parseErr := parseOpenAIWSFirstMessagePayload([]byte(`{"input":"hello"}`))

		require.Nil(t, first)
		require.NotNil(t, parseErr)
		require.Equal(t, "model is required in first response.create payload", parseErr.Reason)
	})

	t.Run("rejects message id previous response", func(t *testing.T) {
		first, parseErr := parseOpenAIWSFirstMessagePayload([]byte(`{"model":"gpt-5.1","previous_response_id":"msg_123"}`))

		require.Nil(t, first)
		require.NotNil(t, parseErr)
		require.Equal(t, coderws.StatusPolicyViolation, parseErr.Status)
		require.Equal(t, "previous_response_id must be a response.id (resp_*), not a message id", parseErr.Reason)
	})
}

func TestOpenAIWSTurnSlotsAcquireNextTurnReleasesAndReacquires(t *testing.T) {
	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
			require.Equal(t, int64(1001), userID)
			require.Equal(t, 2, maxConcurrency)
			return true, nil
		},
		acquireAccountSlotFn: func(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
			require.Equal(t, int64(2002), accountID)
			require.Equal(t, 3, maxConcurrency)
			return true, nil
		},
	}
	h := &OpenAIGatewayHandler{
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
	}
	slots := newOpenAIWSTurnSlots(h, context.Background(), nil, middleware.AuthSubject{
		UserID:      1001,
		Concurrency: 2,
	})
	slots.user = wrapReleaseOnDone(context.Background(), func() {
		atomic.AddInt32(&cache.releaseUserCalled, 1)
	})
	slots.account = wrapReleaseOnDone(context.Background(), func() {
		atomic.AddInt32(&cache.releaseAccountCalled, 1)
	})
	slots.accountID = 2002
	slots.accountMax = 3

	require.NoError(t, slots.acquireNextTurn())

	require.Equal(t, int32(1), atomic.LoadInt32(&cache.releaseUserCalled), "old user slot should be released before acquiring next turn")
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.releaseAccountCalled), "old account slot should be released before acquiring next turn")
	require.NotNil(t, slots.user)
	require.NotNil(t, slots.account)

	slots.releaseAll()
	require.Equal(t, int32(2), atomic.LoadInt32(&cache.releaseUserCalled), "new user slot should be releasable")
	require.Equal(t, int32(2), atomic.LoadInt32(&cache.releaseAccountCalled), "new account slot should be releasable")
}
