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

type openAILatencyAdminStub struct {
	service.AdminService
	tests int
	reads int
}

func (s *openAILatencyAdminStub) GetOpenAIProxyLatency(context.Context, int64) (*service.OpenAIProxyLatency, error) {
	s.reads++
	return nil, nil
}
func (s *openAILatencyAdminStub) TestOpenAIProxyLatency(context.Context, int64) (*service.OpenAIProxyLatency, error) {
	s.tests++
	return &service.OpenAIProxyLatency{Success: true, HTTPStatus: 401}, nil
}

func TestOpenAIProxyLatencyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &openAILatencyAdminStub{}
	router := gin.New()
	NewProxyHandler(svc).RegisterOpenAILatency(router.Group("/proxies"))
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/proxies/11/openai-latency", 200},
		{http.MethodPost, "/proxies/11/openai-latency", 200},
		{http.MethodPost, "/proxies/0/openai-latency", 400},
		{http.MethodPost, "/proxies/-1/openai-latency", 400},
		{http.MethodPost, "/proxies/bad/openai-latency", 400},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
		require.Equal(t, tc.status, recorder.Code)
	}
	require.Equal(t, 1, svc.reads)
	require.Equal(t, 1, svc.tests)
}
