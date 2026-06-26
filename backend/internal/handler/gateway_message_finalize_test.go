package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayMessageBindCache struct {
	groupID    int64
	sessionKey string
	accountID  int64
	calls      int
}

func (c *gatewayMessageBindCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	return 0, nil
}

func (c *gatewayMessageBindCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	c.groupID = groupID
	c.sessionKey = sessionHash
	c.accountID = accountID
	c.calls++
	return nil
}

func (c *gatewayMessageBindCache) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	return nil
}

func (c *gatewayMessageBindCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	return nil
}

func TestBindGatewayMessageStickySession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("binds first successful account", func(t *testing.T) {
		cache := &gatewayMessageBindCache{}
		h := &GatewayHandler{gatewayService: newGatewayMessageBindServiceForTest(cache)}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
		groupID := int64(42)
		req := &gatewayMessageRequest{SessionKey: "session-a"}
		route := &gatewayAnthropicRoute{APIKey: &service.APIKey{GroupID: &groupID}}

		h.bindGatewayMessageStickySession(c, req, route, &service.Account{ID: 9001})

		require.Equal(t, 1, cache.calls)
		require.Equal(t, groupID, cache.groupID)
		require.Equal(t, "session-a", cache.sessionKey)
		require.Equal(t, int64(9001), cache.accountID)
	})

	t.Run("does not overwrite different existing sticky account", func(t *testing.T) {
		cache := &gatewayMessageBindCache{}
		h := &GatewayHandler{gatewayService: newGatewayMessageBindServiceForTest(cache)}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
		groupID := int64(42)
		req := &gatewayMessageRequest{SessionKey: "session-a", SessionBoundAccount: 9001}
		route := &gatewayAnthropicRoute{APIKey: &service.APIKey{GroupID: &groupID}}

		h.bindGatewayMessageStickySession(c, req, route, &service.Account{ID: 9002})

		require.Zero(t, cache.calls)
	})
}

func newGatewayMessageBindServiceForTest(cache service.GatewayCache) *service.GatewayService {
	return service.NewGatewayService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cache,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
}
