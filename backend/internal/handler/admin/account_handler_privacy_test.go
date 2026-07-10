//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type privacyAdminService struct {
	*stubAdminService
	account *service.Account
	mode    string
}

func (s *privacyAdminService) GetAccount(context.Context, int64) (*service.Account, error) {
	return s.account, nil
}

func (s *privacyAdminService) ForceOpenAIPrivacy(context.Context, *service.Account) string {
	return s.mode
}

func TestAccountHandlerSetPrivacyReportsUpstreamFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mode        string
		wantMessage string
	}{
		{
			name:        "edge blocked",
			mode:        service.PrivacyModeCFBlocked,
			wantMessage: "Failed to set privacy: upstream request was blocked; check the account proxy",
		},
		{
			name:        "upstream failed",
			mode:        service.PrivacyModeFailed,
			wantMessage: "Failed to set privacy at upstream",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			gin.SetMode(gin.TestMode)
			adminService := &privacyAdminService{
				stubAdminService: newStubAdminService(),
				account: &service.Account{
					ID:          3,
					Platform:    service.PlatformOpenAI,
					Type:        service.AccountTypeOAuth,
					Credentials: map[string]any{"access_token": "token"},
				},
				mode: test.mode,
			}
			handler := NewAccountHandler(adminService, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.POST("/api/v1/admin/accounts/:id/set-privacy", handler.SetPrivacy)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/3/set-privacy", nil)
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusBadGateway, recorder.Code)
			var payload response.Response
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			require.Equal(t, http.StatusBadGateway, payload.Code)
			require.Equal(t, test.wantMessage, payload.Message)
		})
	}
}
