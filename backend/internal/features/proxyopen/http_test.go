package proxyopen

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutesInheritAdminBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/proxies", func(c *gin.Context) {
		switch c.GetHeader("Test-Role") {
		case "":
			c.AbortWithStatus(401)
		case "user":
			c.AbortWithStatus(403)
		}
	})
	(&Runner{}).Register(group)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, role := range []string{"", "user"} {
			for _, path := range []string{"/proxies/1/openai-monitor", "/proxies/1/openai-monitor/config"} {
				if method == http.MethodPut && strings.HasSuffix(path, "/config") {
					continue
				}
				req := httptest.NewRequest(method, path, strings.NewReader(`{"enabled":true,"interval_seconds":10}`))
				req.Header.Set("Test-Role", role)
				out := httptest.NewRecorder()
				engine.ServeHTTP(out, req)
				expected := 401
				if role == "user" {
					expected = 403
				}
				require.Equal(t, expected, out.Code)
			}
		}
	}
	for _, path := range []string{"/proxies/0/openai-monitor", "/proxies/bad/openai-monitor"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Test-Role", "admin")
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, req)
		require.Equal(t, 400, out.Code)
	}
	req := httptest.NewRequest(http.MethodPut, "/proxies/1/openai-monitor", strings.NewReader("invalid"))
	req.Header.Set("Test-Role", "admin")
	out := httptest.NewRecorder()
	engine.ServeHTTP(out, req)
	require.Equal(t, 400, out.Code)
}

func TestConfigHTTPWorksWhenHistoryFails(t *testing.T) {
	s := testStore(t)
	enable(t, s, 1)
	_, err := s.DB.Exec(`DROP TABLE proxy_openai_monitor_samples`)
	require.NoError(t, err)
	engine := gin.New()
	New(s, nil).Register(engine.Group("/proxies"))
	request := func(method, path, body string) *httptest.ResponseRecorder {
		out := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(out, req)
		return out
	}
	require.Equal(t, 500, request("GET", "/proxies/1/openai-monitor", "").Code)
	out := request("GET", "/proxies/1/openai-monitor/config", "")
	require.Equal(t, 200, out.Code)
	var response struct {
		Data Settings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Body.Bytes(), &response))
	require.True(t, response.Data.Config.Enabled)
	require.Equal(t, 200, request("PUT", "/proxies/1/openai-monitor", `{"enabled":false,"interval_seconds":10}`).Code)
	settings, err := s.ReadConfig(context.Background(), 1)
	require.NoError(t, err)
	require.False(t, settings.Config.Enabled)
	require.Equal(t, 400, request("GET", "/proxies/1/openai-monitor?until=invalid", "").Code)
}
