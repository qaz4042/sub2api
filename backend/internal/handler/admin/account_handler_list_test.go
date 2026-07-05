package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupAccountListRouter() (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.GET("/api/v1/admin/accounts", handler.List)
	return router, adminSvc
}

func TestAccountHandlerListIncludesCreatedAt(t *testing.T) {
	router, adminSvc := setupAccountListRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=20&sort_by=created_at&sort_order=desc", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "created_at", adminSvc.lastListAccounts.sortBy)

	var payload struct {
		Data struct {
			Items []struct {
				ID        int64  `json:"id"`
				CreatedAt string `json:"created_at"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Items, 1)

	createdAt := payload.Data.Items[0].CreatedAt
	require.NotEmpty(t, createdAt)
	require.True(t, strings.HasSuffix(createdAt, "Z"), "created_at should be serialized as UTC")
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	require.NoError(t, err)
	_, offset := parsed.Zone()
	require.Equal(t, 0, offset)
}

func TestShouldAutoRefreshOpenAISubscription(t *testing.T) {
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		account *service.Account
		want    bool
	}{
		{
			name: "missing expiry for paid plan",
			account: &service.Account{
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeOAuth,
				Credentials: map[string]any{
					"access_token": "token",
					"plan_type":    "plus",
				},
			},
			want: true,
		},
		{
			name: "missing expiry for free plan",
			account: &service.Account{
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeOAuth,
				Credentials: map[string]any{
					"access_token": "token",
					"plan_type":    "free",
				},
			},
			want: false,
		},
		{
			name: "near expiry",
			account: &service.Account{
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeOAuth,
				Credentials: map[string]any{
					"access_token":            "token",
					"plan_type":               "plus",
					"subscription_expires_at": now.Add(3 * 24 * time.Hour).Format(time.RFC3339),
				},
			},
			want: true,
		},
		{
			name: "far expiry",
			account: &service.Account{
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeOAuth,
				Credentials: map[string]any{
					"access_token":            "token",
					"plan_type":               "plus",
					"subscription_expires_at": now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
				},
			},
			want: false,
		},
		{
			name: "no access token",
			account: &service.Account{
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeOAuth,
				Credentials: map[string]any{
					"plan_type": "plus",
				},
			},
			want: false,
		},
		{
			name: "non openai oauth",
			account: &service.Account{
				Platform: service.PlatformAnthropic,
				Type:     service.AccountTypeOAuth,
				Credentials: map[string]any{
					"access_token": "token",
					"plan_type":    "plus",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shouldAutoRefreshOpenAISubscription(tt.account, now))
		})
	}
}

func TestAccountHandlerTryMarkSubscriptionRefreshCooldown(t *testing.T) {
	handler := NewAccountHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)

	require.True(t, handler.tryMarkSubscriptionRefresh(42, now))
	require.False(t, handler.tryMarkSubscriptionRefresh(42, now.Add(time.Hour)))
	require.True(t, handler.tryMarkSubscriptionRefresh(42, now.Add(openAISubscriptionAutoRefreshCooldown+time.Second)))
}
