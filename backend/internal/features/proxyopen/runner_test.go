package proxyopen

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestIntervalsAndFixedPhase(t *testing.T) {
	for _, n := range []int{10, 30, 60} {
		require.True(t, validInterval(n))
	}
	for _, n := range []int{0, -1, 11, 20, 120} {
		require.False(t, validInterval(n))
	}
	plan := time.Unix(100, 0)
	for _, tc := range []struct {
		elapsed time.Duration
		want    int64
	}{{0, 110}, {9 * time.Second, 110}, {10 * time.Second, 120}, {21 * time.Second, 130}, {time.Hour, 3710}} {
		require.Equal(t, time.Unix(tc.want, 0), nextPlan(plan, plan.Add(tc.elapsed), 10))
	}
	require.Equal(t, plan, nextPlan(plan, plan.Add(-time.Second), 10))
}
func TestClassification(t *testing.T) {
	for _, tc := range []struct {
		status  int
		err     error
		success bool
		class   string
	}{
		{200, nil, true, ""}, {401, nil, true, ""}, {403, nil, false, "http_403"}, {429, nil, false, "http_429"}, {503, nil, false, "http_5xx"}, {302, nil, false, "http_other"},
		{0, fmt.Errorf("proxy secret: %w", context.DeadlineExceeded), false, "timeout"}, {0, fmt.Errorf("sensitive credentials"), false, "connection"},
	} {
		success, class := classify(tc.status, tc.err)
		require.Equal(t, tc.success, success)
		require.Equal(t, tc.class, class)
	}
}
