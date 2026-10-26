package admin

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountStatsRejectInvalidGranularity(t *testing.T) {
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Params = gin.Params{{Key: "id", Value: "7"}}
	ctx.Request = httptest.NewRequest("GET", "/?granularity=minute", nil)
	(&AccountHandler{}).GetStats(ctx)
	require.Equal(t, 400, response.Code)
	require.Contains(t, response.Body.String(), "granularity must be day or hour")
}

func TestAccountStatsRejectInvalidIncludeTests(t *testing.T) {
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Params = gin.Params{{Key: "id", Value: "7"}}
	ctx.Request = httptest.NewRequest("GET", "/?include_tests=maybe", nil)
	(&AccountHandler{}).GetStats(ctx)
	require.Equal(t, 400, response.Code)
	require.Contains(t, response.Body.String(), "include_tests must be a boolean")
}
