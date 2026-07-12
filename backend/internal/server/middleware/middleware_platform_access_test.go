//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type platformAccessSettingRepoStub struct{}

func (platformAccessSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}
func (platformAccessSettingRepoStub) GetValue(context.Context, string) (string, error) {
	return "", service.ErrSettingNotFound
}
func (platformAccessSettingRepoStub) Set(context.Context, string, string) error { return nil }
func (platformAccessSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (platformAccessSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (platformAccessSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (platformAccessSettingRepoStub) Delete(context.Context, string) error { return nil }

type platformAccessConfigRepoStub struct{}

func (platformAccessConfigRepoStub) List(context.Context) ([]service.PlatformConfig, error) {
	return []service.PlatformConfig{
		{Key: service.PlatformOpenAI, Enabled: true, Core: true},
		{Key: service.PlatformAnthropic, Enabled: false},
	}, nil
}
func (platformAccessConfigRepoStub) Get(context.Context, string) (*service.PlatformConfig, error) {
	return nil, service.ErrPlatformConfigNotFound
}
func (platformAccessConfigRepoStub) SetEnabled(context.Context, string, bool) (*service.PlatformConfig, error) {
	return nil, nil
}

func TestRequirePlatformEnabledRejectsDisabledGroupPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settingService := service.NewSettingService(platformAccessSettingRepoStub{}, &config.Config{})
	settingService.SetPlatformConfigRepository(platformAccessConfigRepoStub{})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(1)
		c.Set(string(ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{Platform: service.PlatformAnthropic}})
		c.Next()
	})
	router.Use(RequirePlatformEnabled(settingService, AnthropicErrorWriter))
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "disabled")
}
