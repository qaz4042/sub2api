package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type platformGuardSettingRepoStub struct {
	values map[string]string
}

func (s *platformGuardSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}
func (s *platformGuardSettingRepoStub) GetValue(context.Context, string) (string, error) {
	return "", service.ErrSettingNotFound
}
func (s *platformGuardSettingRepoStub) Set(context.Context, string, string) error { return nil }
func (s *platformGuardSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (s *platformGuardSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}
func (s *platformGuardSettingRepoStub) Delete(context.Context, string) error { return nil }
func (s *platformGuardSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func TestRequirePlatformEnabledBlocksDisabledGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewSettingService(&platformGuardSettingRepoStub{values: map[string]string{}}, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{Platform: service.PlatformAnthropic}})
		c.Next()
	})
	router.Use(RequirePlatformEnabled(svc, AnthropicErrorWriter))
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/test", nil))

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "anthropic platform interface is disabled")
}

func TestRequirePlatformEnabledAlwaysAllowsOpenAI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewSettingService(&platformGuardSettingRepoStub{values: map[string]string{}}, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{Platform: service.PlatformOpenAI}})
		c.Next()
	})
	router.Use(RequirePlatformEnabled(svc, AnthropicErrorWriter))
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/test", nil))

	require.Equal(t, http.StatusNoContent, recorder.Code)
}
