package proxyquality

import (
	"github.com/Wei-Shaw/sub2api/internal/features/proxyopen"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func good(id int64, latency float64, now time.Time) proxyopen.Quality {
	first, last := now.Add(-270*time.Second), now.Add(-time.Second)
	return proxyopen.Quality{ID: id, Eligible: true, Enabled: true, Interval: 30, Count: 10, Successes: 10, P95: &latency, First: &first, Last: &last, RecentSuccesses: 3}
}
func TestBusinessPolicy(t *testing.T) {
	now := time.Now()
	slow, fast := 2500.0, 500.0
	baseline := businessSummary{Metrics: []businessMetric{
		{ProxyID: 1, Model: "gpt", Transport: "http", Total: 10, Successes: 10, TTFTCount: 10, Median: &slow, Last: &now, RecentTotal: 10, RecentSuccesses: 10, RecentTTFTCount: 10, RecentMedian: &slow},
		{ProxyID: 2, Model: "gpt", Transport: "http", Total: 10, Successes: 10, TTFTCount: 10, Median: &fast, Last: &now},
	}}
	qs := []proxyopen.Quality{good(1, 2500, now), good(2, 500, now)}
	target, reason := choose(1, qs, baseline, now, nil)
	require.EqualValues(t, 2, target)
	require.Equal(t, "business_improved", reason)
	mismatched := baseline
	mismatched.Metrics = append([]businessMetric(nil), baseline.Metrics...)
	mismatched.Metrics[1].Model = "other"
	target, reason = choose(1, qs, mismatched, now, nil)
	require.Zero(t, target)
	require.Equal(t, "business_insufficient", reason)
	mismatched.Metrics[1].Model = "gpt"
	mismatched.Metrics[1].Transport = "ws"
	target, reason = choose(1, qs, mismatched, now, nil)
	require.Zero(t, target)
	require.Equal(t, "business_insufficient", reason)
	missing := baseline
	missing.Metrics = append([]businessMetric(nil), baseline.Metrics...)
	missing.Metrics[1].TTFTCount = 2
	target, _ = choose(1, qs, missing, now, nil)
	require.Zero(t, target)
	gap := baseline
	gap.Gap = true
	target, reason = choose(1, qs, gap, now, nil)
	require.Zero(t, target)
	require.Equal(t, "collection_unavailable", reason)
	bad := baseline
	bad.Metrics = append([]businessMetric(nil), baseline.Metrics...)
	bad.Metrics[0].RecentSuccesses = 5
	bad.Metrics[0].RecentFailures = 3
	last := now.Add(-2 * time.Minute)
	target, reason = choose(1, qs, bad, now, &last)
	require.EqualValues(t, 2, target)
	require.Equal(t, "business_degraded", reason)
	last = now.Add(-30 * time.Second)
	target, reason = choose(1, qs, bad, now, &last)
	require.Zero(t, target)
	require.Equal(t, "cooldown", reason)
	last = now.Add(-2 * time.Minute)
	target, reason = choose(1, qs, baseline, now, &last)
	require.Zero(t, target)
	require.Equal(t, "cooldown", reason)
	persistent := baseline
	persistent.Metrics = append([]businessMetric(nil), baseline.Metrics...)
	persistent.Metrics[0].RecentTotal = 1
	persistent.Metrics[0].RecentSuccesses = 0
	persistent.Metrics[0].RecentPersistent = true
	target, reason = choose(1, qs, persistent, now, nil)
	require.EqualValues(t, 2, target)
	require.Equal(t, "business_degraded", reason)
	qs[0].Eligible = false
	target, reason = choose(1, qs, baseline, now, nil)
	require.EqualValues(t, 2, target)
	require.Equal(t, "business_degraded", reason)
	qs[0].Eligible = true
	qs[1].Successes = 8
	require.True(t, qualified(qs[1], now))
	qs[1].RecentSuccesses = 2
	require.False(t, qualified(qs[1], now))
}

func TestFailureMustMatchCohortAndHTTP5xxNeedsRecentCandidateSuccess(t *testing.T) {
	now := time.Now()
	slow, fast := 2500.0, 500.0
	first := now.Add(-time.Minute)
	success := now.Add(-30 * time.Second)
	qs := []proxyopen.Quality{good(1, 1500, now), good(2, 300, now)}
	actual := businessSummary{Metrics: []businessMetric{
		{ProxyID: 1, Model: "failed", Transport: "http", RecentTotal: 3, RecentFailures: 3},
		{ProxyID: 1, Model: "healthy", Transport: "http", RecentTotal: 1},
		{ProxyID: 2, Model: "healthy", Transport: "http", Total: 5, Successes: 5, Last: &now},
	}}
	target, _ := choose(1, qs, actual, now, nil)
	require.Zero(t, target)
	actual.Metrics = []businessMetric{
		{ProxyID: 1, Model: "gpt", Transport: "http", RecentHTTP5xx: 3, FirstHTTP5xx: &first},
		{ProxyID: 2, Model: "gpt", Transport: "http", Total: 5, Successes: 5, TTFTCount: 3, Median: &fast, Last: &now, RecentSuccesses: 2, LastRecentSuccess: &success},
	}
	target, reason := choose(1, qs, actual, now, nil)
	require.EqualValues(t, 2, target)
	require.Equal(t, "business_5xx", reason)
	older := first.Add(-time.Second)
	actual.Metrics[1].LastRecentSuccess = &older
	target, _ = choose(1, qs, actual, now, nil)
	require.Zero(t, target)
	actual.Metrics[1].LastRecentSuccess = &success
	actual.Metrics[1].RecentSuccesses = 1
	target, _ = choose(1, qs, actual, now, nil)
	require.Zero(t, target)
	_ = slow
}
