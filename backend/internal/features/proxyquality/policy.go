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

// Business observations drive fault handling; probes drive normal optimization.
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
	// An ineligible current connection is an explicit availability failure. A
	// fresh, healthy monitor candidate may take over even without account
	// history; the first real request after the switch supplies that history.
	if unavailable {
		return fastestCandidate(quality, candidates), "business_degraded"
	}
	var failureID int64
	var coldFailureID int64
	var waitingFor5xxEvidence bool
	failureReason := ""
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
		waitingFor5xxEvidence = waitingFor5xxEvidence || serverDegraded
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
		}
		// Transport failures are sufficient to use a monitor-only candidate.
		// HTTP 5xx remains history-gated because it may be account/model-specific.
		if degraded && coldFailureID == 0 {
			coldFailureID = fastestCandidate(quality, candidates)
		}
	}
	if failureID != 0 {
		return failureID, failureReason
	}
	if coldFailureID != 0 {
		return coldFailureID, "business_degraded"
	}
	if waitingFor5xxEvidence {
		return 0, "business_5xx_waiting"
	}
	if cooling {
		return 0, "cooldown"
	}
	return 0, "business_insufficient"
}

func fastestCandidate(quality []proxyopen.Quality, candidates map[int64]bool) int64 {
	var result int64
	var latency float64
	for _, q := range quality {
		if candidates[q.ID] && q.P95 != nil && (result == 0 || *q.P95 < latency || (*q.P95 == latency && q.ID < result)) {
			result, latency = q.ID, *q.P95
		}
	}
	return result
}

// Counts healthy monitor candidates; switch triggers and cooldown still apply.
func candidateCount(current int64, quality []proxyopen.Quality, now time.Time) int {
	count := 0
	for _, q := range quality {
		if q.ID != current && qualified(q, now) {
			count++
		}
	}
	return count
}

type monitorConfirmation struct {
	CandidateID                              int64
	Count                                    int
	CurrentAt, CandidateAt                   *time.Time
	CurrentFingerprint, CandidateFingerprint string
}

// Each confirmation uses new completed probes from both connections.
func confirmMonitor(current int64, quality []proxyopen.Quality, now time.Time, state monitorConfirmation) (int64, monitorConfirmation) {
	var source *proxyopen.Quality
	for i := range quality {
		if quality[i].ID == current {
			source = &quality[i]
			break
		}
	}
	if source == nil || !qualified(*source, now) {
		return 0, monitorConfirmation{}
	}
	var best *proxyopen.Quality
	for i := range quality {
		q := &quality[i]
		if q.ID == current || !qualified(*q, now) || *source.P95-*q.P95 < 200 || *q.P95 > *source.P95*0.7 {
			continue
		}
		if best == nil || *q.P95 < *best.P95 || (*q.P95 == *best.P95 && q.ID < best.ID) {
			best = q
		}
	}
	if best == nil {
		return 0, monitorConfirmation{}
	}
	if state.CandidateID != best.ID || state.CurrentFingerprint != source.Fingerprint || state.CandidateFingerprint != best.Fingerprint || state.CurrentAt == nil || state.CandidateAt == nil {
		state = monitorConfirmation{CandidateID: best.ID, Count: 1, CurrentAt: source.Last, CandidateAt: best.Last, CurrentFingerprint: source.Fingerprint, CandidateFingerprint: best.Fingerprint}
	} else if source.Last.After(*state.CurrentAt) && best.Last.After(*state.CandidateAt) {
		state.Count++
		state.CurrentAt, state.CandidateAt = source.Last, best.Last
	}
	if state.Count >= 3 {
		return best.ID, monitorConfirmation{}
	}
	return 0, state
}
