package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"
)

func TestFetchChatGPTSubscriptionExpiresAt(t *testing.T) {
	const wantExpiresAt = "2026-06-10T02:52:15Z"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/backend-api/subscriptions", r.URL.Path)
		require.Equal(t, "acc_123", r.URL.Query().Get("account_id"))
		require.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"plan_type":    "plus",
			"active_until": wantExpiresAt,
			"will_renew":   true,
			"id":           "sub_123",
		})
	}))
	defer server.Close()

	oldURL := chatGPTSubscriptionsURL
	chatGPTSubscriptionsURL = server.URL + "/backend-api/subscriptions"
	t.Cleanup(func() { chatGPTSubscriptionsURL = oldURL })

	got := fetchChatGPTSubscriptionExpiresAt(context.Background(), func(proxyURL string) (*req.Client, error) {
		return req.C().SetTimeout(5 * time.Second), nil
	}, "access-token", "", "acc_123")

	require.Equal(t, wantExpiresAt, got)
}

func TestOpenAIOAuthService_RefreshAccountSubscription_UsesExistingAccessToken(t *testing.T) {
	const wantExpiresAt = "2026-06-10T02:52:15Z"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))

		switch r.URL.Path {
		case "/backend-api/accounts/check/v4-2023-04-27":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"accounts": map[string]any{
					"acc_123": map[string]any{
						"account": map[string]any{
							"plan_type":  "plus",
							"is_default": true,
						},
					},
				},
			})
		case "/backend-api/subscriptions":
			require.Equal(t, "acc_123", r.URL.Query().Get("account_id"))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"plan_type":    "plus",
				"active_until": wantExpiresAt,
				"will_renew":   true,
				"id":           "sub_123",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldAccountsURL := chatGPTAccountsCheckURL
	oldSubscriptionsURL := chatGPTSubscriptionsURL
	chatGPTAccountsCheckURL = server.URL + "/backend-api/accounts/check/v4-2023-04-27"
	chatGPTSubscriptionsURL = server.URL + "/backend-api/subscriptions"
	t.Cleanup(func() {
		chatGPTAccountsCheckURL = oldAccountsURL
		chatGPTSubscriptionsURL = oldSubscriptionsURL
	})

	client := &openaiOAuthClientRefreshStub{}
	svc := NewOpenAIOAuthService(nil, client)
	svc.SetPrivacyClientFactory(func(proxyURL string) (*req.Client, error) {
		return req.C().SetTimeout(5 * time.Second), nil
	})

	got, err := svc.RefreshAccountSubscription(context.Background(), &Account{
		ID:       77,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "access-token",
			"chatgpt_account_id": "acc_123",
			"refresh_token":      "refresh-token",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "plus", got.PlanType)
	require.Equal(t, wantExpiresAt, got.SubscriptionExpiresAt)
	require.Zero(t, atomic.LoadInt32(&client.refreshCalls), "subscription refresh must not refresh OAuth token")
}
