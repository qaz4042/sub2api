//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scheduledTestRunRecorder struct {
	ScheduledTestPlanRepository
	planID    int64
	lastRunAt *time.Time
	nextRunAt time.Time
}

func (r *scheduledTestRunRecorder) UpdateAfterRun(_ context.Context, id int64, lastRunAt *time.Time, nextRunAt time.Time) error {
	r.planID = id
	r.lastRunAt = lastRunAt
	r.nextRunAt = nextRunAt
	return nil
}

func TestScheduledTestRunnerSkipsRateLimitedAccount(t *testing.T) {
	now := time.Now()
	reset := now.Add(time.Hour)
	account := &Account{ID: 77, RateLimitResetAt: &reset}
	planRepo := &scheduledTestRunRecorder{}
	runner := &ScheduledTestRunnerService{
		planRepo:       planRepo,
		accountTestSvc: &AccountTestService{accountRepo: &accountTestLoggingRepo{account: account}},
	}

	// The skip path needs neither the upstream client nor the result service.
	runner.runOnePlan(context.Background(), &ScheduledTestPlan{
		ID:             2,
		AccountID:      account.ID,
		CronExpression: "* * * * *",
		AutoRecover:    true,
	})

	require.Equal(t, int64(2), planRepo.planID)
	require.Nil(t, planRepo.lastRunAt)
	require.True(t, planRepo.nextRunAt.After(now))
	require.True(t, planRepo.nextRunAt.Before(reset))
}

func TestScheduledTestBackgroundRespectsOpenAIQuotaReset(t *testing.T) {
	ctx := context.Background()
	reset := time.Now().Add(time.Hour).Truncate(time.Second)
	account := &Account{
		ID:          77,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Credentials: map[string]any{"access_token": "test-token"},
	}
	repo := &openAIAccountTestRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}},
	}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusTooManyRequests, fmt.Sprintf(`{"error":{"type":"usage_limit_reached","resets_at":%d}}`, reset.Unix())),
		newJSONResponse(http.StatusOK, "data: {\"type\":\"response.completed\"}\n\n"),
	}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}

	result, err := svc.RunTestBackground(ctx, account.ID, "gpt-5.4")
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
	require.NotNil(t, repo.rateLimitedAt)
	require.WithinDuration(t, reset, *repo.rateLimitedAt, 0)

	result, err = svc.RunTestBackground(ctx, account.ID, "gpt-5.4")
	require.ErrorIs(t, err, errScheduledTestRateLimited)
	require.Nil(t, result)
	require.Len(t, upstream.requests, 1)

	expired := time.Now().Add(-time.Minute)
	account.RateLimitResetAt = &expired
	result, err = svc.RunTestBackground(ctx, account.ID, "gpt-5.4")
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.Len(t, upstream.requests, 2)
}
