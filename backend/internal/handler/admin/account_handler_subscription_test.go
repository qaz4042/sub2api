package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type subscriptionAdminService struct {
	*stubAdminService
	account *service.Account
}

func (s *subscriptionAdminService) GetAccount(context.Context, int64) (*service.Account, error) {
	return s.account, nil
}

func TestAccountHandlerRefreshSubscriptionRejectsNonOpenAIAccount(t *testing.T) {
	adminService := &subscriptionAdminService{
		stubAdminService: newStubAdminService(),
		account: &service.Account{
			ID:       3,
			Platform: service.PlatformAntigravity,
			Type:     service.AccountTypeOAuth,
		},
	}
	handler := NewAccountHandler(adminService, nil, service.NewOpenAIOAuthService(nil, nil), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/refresh-subscription", handler.RefreshSubscription)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/3/refresh-subscription", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAccountHandlerRefreshSubscriptionRejectsShadowAccount(t *testing.T) {
	parentID := int64(11)
	adminService := &subscriptionAdminService{
		stubAdminService: newStubAdminService(),
		account: &service.Account{
			ID:              3,
			Platform:        service.PlatformOpenAI,
			Type:            service.AccountTypeOAuth,
			ParentAccountID: &parentID,
		},
	}
	handler := NewAccountHandler(adminService, nil, service.NewOpenAIOAuthService(nil, nil), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/refresh-subscription", handler.RefreshSubscription)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/3/refresh-subscription", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
