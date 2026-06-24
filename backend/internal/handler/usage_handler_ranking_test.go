package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userRankingRepoStub struct {
	service.UsageLogRepository
	result *usagestats.UserAPIKeyRankingResponse
	userID int64
	limit  int
}

func (s *userRankingRepoStub) GetUserAPIKeySpendingRanking(
	ctx context.Context,
	startTime, endTime time.Time,
	userID int64,
	limit int,
) (*usagestats.UserAPIKeyRankingResponse, error) {
	s.userID = userID
	s.limit = limit
	return s.result, nil
}

func newUserRankingTestRouter(repo *userRankingRepoStub, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		c.Next()
	})
	router.GET("/usage/ranking", handler.APIKeyRanking)
	return router
}

func TestAPIKeyRankingRedactsOtherUsersAndReturnsOwnRank(t *testing.T) {
	repo := &userRankingRepoStub{result: &usagestats.UserAPIKeyRankingResponse{
		Ranking: []usagestats.UserAPIKeyRankingItem{
			{Rank: 1, APIKeyID: 11, KeyName: "secret-key", UserID: 9, ActualCost: 12},
			{Rank: 2, APIKeyID: 22, KeyName: "mine", UserID: 42, IsMine: true, ActualCost: 8},
		},
		MyRankings: []usagestats.UserAPIKeyRankingItem{
			{Rank: 2, APIKeyID: 22, KeyName: "mine", UserID: 42, IsMine: true, ActualCost: 8},
			{Rank: 18, APIKeyID: 23, KeyName: "my-other-key", UserID: 42, IsMine: true, ActualCost: 1},
		},
		TotalKeys: 30,
	}}
	router := newUserRankingTestRouter(repo, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/ranking?limit=10&start_date=2026-06-01&end_date=2026-06-07", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "private, max-age=30", rec.Header().Get("Cache-Control"))
	require.Equal(t, int64(42), repo.userID)
	require.Equal(t, 10, repo.limit)

	var body struct {
		Data struct {
			Ranking    []usagestats.UserAPIKeyRankingItem `json:"ranking"`
			MyRankings []usagestats.UserAPIKeyRankingItem `json:"my_rankings"`
			TotalKeys  int64                              `json:"total_keys"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, int64(30), body.Data.TotalKeys)
	require.Zero(t, body.Data.Ranking[0].APIKeyID)
	require.Equal(t, "s******y", body.Data.Ranking[0].KeyName)
	require.Equal(t, int64(22), body.Data.Ranking[1].APIKeyID)
	require.Equal(t, "mine", body.Data.Ranking[1].KeyName)
	require.Len(t, body.Data.MyRankings, 2)
	require.Equal(t, int64(18), body.Data.MyRankings[1].Rank)
}

func TestAPIKeyRankingReturnsEmptyArraysWhenNoRows(t *testing.T) {
	repo := &userRankingRepoStub{result: &usagestats.UserAPIKeyRankingResponse{}}
	router := newUserRankingTestRouter(repo, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/ranking?limit=10", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data struct {
			Ranking    json.RawMessage `json:"ranking"`
			MyRankings json.RawMessage `json:"my_rankings"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.JSONEq(t, "[]", string(body.Data.Ranking))
	require.JSONEq(t, "[]", string(body.Data.MyRankings))
}

func TestMaskAPIKeyName(t *testing.T) {
	require.Equal(t, "", maskAPIKeyName(""))
	require.Equal(t, "*", maskAPIKeyName("a"))
	require.Equal(t, "a*", maskAPIKeyName("ab"))
	require.Equal(t, "p******n", maskAPIKeyName("production"))
	require.Equal(t, "测**称", maskAPIKeyName("测试名称"))
}

func TestAPIKeyRankingRejectsInvalidLimit(t *testing.T) {
	repo := &userRankingRepoStub{result: &usagestats.UserAPIKeyRankingResponse{}}
	router := newUserRankingTestRouter(repo, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/ranking?limit=100", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPIKeyRankingRejectsInvalidOrExcessiveDateRange(t *testing.T) {
	repo := &userRankingRepoStub{result: &usagestats.UserAPIKeyRankingResponse{}}
	router := newUserRankingTestRouter(repo, 42)

	for _, path := range []string{
		"/usage/ranking?start_date=invalid&end_date=2026-06-07",
		"/usage/ranking?start_date=2026-06-07&end_date=2026-06-01",
		"/usage/ranking?start_date=2026-01-01&end_date=2026-06-01",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, path)
	}
}
