// Package proxyquality owns opt-in account proxy routing.
package proxyquality

import (
	"github.com/Wei-Shaw/sub2api/internal/features/proxyopen"
	"time"
)

const cooldown = 5 * time.Minute
const emergencyCooldown = time.Minute

func fresh(q proxyopen.Quality, now time.Time) bool {
	return q.Last != nil && !q.Last.After(now) && now.Sub(*q.Last) <= time.Duration(q.Interval*3+16)*time.Second
}
func enough(q proxyopen.Quality) bool {
	return q.Interval > 0 && q.Count >= 5 && q.Count*q.Interval >= 180 && q.First != nil && q.Last != nil && q.Last.Sub(*q.First) >= 2*time.Minute
}
func qualified(q proxyopen.Quality, now time.Time) bool {
	return q.Eligible && q.Enabled && fresh(q, now) && enough(q) && q.Successes*100 >= q.Count*80 && q.RecentSuccesses == 3 && q.P95 != nil
}

type businessMetric struct {
	ProxyID                                       int64
	Model                                         string
	Transport                                     string
	Total, Successes, TTFTCount                   int
	Median                                        *float64
	Last                                          *time.Time
	RecentTotal, RecentSuccesses, RecentTTFTCount int
	RecentMedian                                  *float64
	LastRecentSuccess                             *time.Time
	RecentFailures, RecentHTTP5xx                 int
	RecentPersistent                              bool
	FirstHTTP5xx                                  *time.Time
}
type businessSummary struct {
	Metrics    []businessMetric
	Gap        bool
	GapVersion int64
}

// The probe is a candidate reachability guard; it cannot supply model speed or
// substitute for missing business observations.
func choose(current int64, quality []proxyopen.Quality, actual businessSummary, now time.Time, lastSwitch *time.Time) (int64, string) {
	if actual.Gap {
		return 0, "collection_unavailable"
	}
	unavailable := false
	for _, q := range quality {
		if q.ID == current && !q.Eligible {
			unavailable = true
			break
		}
	}
	elapsed := cooldown
	if lastSwitch != nil {
		elapsed = now.Sub(*lastSwitch)
	}
	if elapsed < emergencyCooldown {
		return 0, "cooldown"
	}
	candidates := map[int64]bool{}
	for _, q := range quality {
		if q.ID != current && qualified(q, now) {
			candidates[q.ID] = true
		}
	}
	if len(candidates) == 0 {
		return 0, "no_candidate"
	}
	var bestID int64
	var failureID int64
	failureReason := ""
	bestImprovement := 0.0
	compared := false
	cooling := elapsed < cooldown
	for _, currentMetric := range actual.Metrics {
		if currentMetric.ProxyID != current || currentMetric.Transport == "" {
			continue
		}
		if currentMetric.RecentTotal == 0 && currentMetric.RecentHTTP5xx == 0 {
			continue
		}
		degraded := unavailable || currentMetric.RecentPersistent || currentMetric.RecentFailures >= 3 || (currentMetric.RecentTotal >= 5 && currentMetric.RecentSuccesses*100 < currentMetric.RecentTotal*80)
		serverDegraded := currentMetric.RecentHTTP5xx >= 3 && currentMetric.FirstHTTP5xx != nil
		if !degraded && !serverDegraded && currentMetric.RecentTotal < 5 {
			continue
		}
		if !degraded && !serverDegraded && elapsed < cooldown {
			cooling = true
			continue
		}
		for _, target := range actual.Metrics {
			if !candidates[target.ProxyID] || target.Model != currentMetric.Model || target.Transport != currentMetric.Transport || target.Total < 5 || target.Successes*100 < target.Total*80 || target.Last == nil {
				continue
			}
			if degraded {
				if failureID == 0 || target.ProxyID < failureID {
					failureID = target.ProxyID
					failureReason = "business_degraded"
				}
				continue
			}
			if serverDegraded {
				if target.RecentSuccesses >= 2 && target.LastRecentSuccess != nil && target.LastRecentSuccess.After(*currentMetric.FirstHTTP5xx) {
					if failureID == 0 || target.ProxyID < failureID {
						failureID = target.ProxyID
						failureReason = "business_5xx"
					}
				}
				continue
			}
			if currentMetric.RecentTTFTCount < 3 || currentMetric.RecentMedian == nil || target.TTFTCount < 3 || target.Median == nil {
				continue
			}
			compared = true
			improvement := *currentMetric.RecentMedian - *target.Median
			if improvement >= 200 && *target.Median <= *currentMetric.RecentMedian*0.7 && (improvement > bestImprovement || (improvement == bestImprovement && (bestID == 0 || target.ProxyID < bestID))) {
				bestID = target.ProxyID
				bestImprovement = improvement
			}
		}
	}
	if failureID != 0 {
		return failureID, failureReason
	}
	if bestID != 0 {
		return bestID, "business_improved"
	}
	if cooling {
		return 0, "cooldown"
	}
	if compared {
		return 0, "business_stable"
	}
	return 0, "business_insufficient"
}
