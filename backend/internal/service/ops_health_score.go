package service

import (
	"math"
	"time"
)

const opsHealthLowSampleThreshold int64 = 30

// computeDashboardHealthScore computes a 0-100 health score from the metrics returned by the dashboard overview.
//
// Design goals:
// - Backend-owned scoring (UI only displays).
// - Layered scoring: Business Health (70%) + Infrastructure Health (30%)
// - Avoids double-counting (e.g., DB failure affects both infra and business metrics)
// - Conservative + stable: penalize clear degradations; avoid overreacting to missing/idle data.
func computeDashboardHealthScore(now time.Time, overview *OpsDashboardOverview) int {
	score, meta := computeDashboardHealthScoreWithMeta(now, overview)
	if overview != nil {
		overview.HealthScoreMeta = meta
	}
	return score
}

func computeDashboardHealthScoreWithMeta(now time.Time, overview *OpsDashboardOverview) (int, *OpsHealthScoreMeta) {
	if overview == nil {
		return 0, nil
	}

	meta := &OpsHealthScoreMeta{
		Confidence:    "normal",
		PrimarySignal: "healthy",
		SampleSize:    overview.RequestCountTotal,
		Reasons:       []string{},
	}

	// Idle/no-data: avoid showing a "bad" score when there is no traffic.
	// UI can still render a gray/idle state based on QPS + error rate.
	if overview.RequestCountSLA <= 0 && overview.RequestCountTotal <= 0 && overview.ErrorCountTotal <= 0 {
		meta.PrimarySignal = "idle"
		meta.BusinessScore = 100
		meta.InfraScore = int(math.Round(computeInfraHealth(now, overview)))
		return 100, meta
	}

	businessHealth := computeBusinessHealth(overview)
	infraHealth := computeInfraHealth(now, overview)

	// Weighted combination: 70% business + 30% infrastructure
	score := int(math.Round(clampFloat64(businessHealth*0.7+infraHealth*0.3, 0, 100)))
	meta.BusinessScore = int(math.Round(businessHealth))
	meta.InfraScore = int(math.Round(infraHealth))
	meta.LowSample = overview.RequestCountTotal > 0 && overview.RequestCountTotal < opsHealthLowSampleThreshold
	if meta.LowSample {
		meta.Confidence = "low_sample"
		meta.Reasons = append(meta.Reasons, "low_sample")
	}
	if value, percentile := healthTTFTReference(overview); value != nil {
		meta.TTFTReferenceMs = value
		meta.TTFTReferencePctl = percentile
	}
	meta.PrimarySignal = determineHealthPrimarySignal(overview, meta)
	return score, meta
}

// computeBusinessHealth calculates business health score (0-100)
// Components: Error Rate (50%) + TTFT (50%)
func computeBusinessHealth(overview *OpsDashboardOverview) float64 {
	// Error rate score: 1% → 100, 10% → 0 (linear)
	// Combines request errors and upstream errors
	errorScore := 100.0
	errorPct := clampFloat64(overview.ErrorRate*100, 0, 100)
	upstreamPct := clampFloat64(overview.UpstreamErrorRate*100, 0, 100)
	combinedErrorPct := math.Max(errorPct, upstreamPct) // Use worst case
	if combinedErrorPct > 1.0 {
		if combinedErrorPct <= 10.0 {
			errorScore = (10.0 - combinedErrorPct) / 9.0 * 100
		} else {
			errorScore = 0
		}
	}

	// TTFT score: 1.5s → 100, 6s → 0 (linear).
	// Use P95 when available; P99 is too jumpy in low-volume windows.
	ttftScore := 100.0
	if reference, _ := healthTTFTReference(overview); reference != nil {
		ttft := float64(*reference)
		if ttft > 1500 {
			if ttft <= 6000 {
				ttftScore = (6000 - ttft) / 4500 * 100
			} else {
				ttftScore = 0
			}
		}
	}

	// Weighted combination: 50% error rate + 50% TTFT
	return errorScore*0.5 + ttftScore*0.5
}

func healthTTFTReference(overview *OpsDashboardOverview) (*int, string) {
	if overview == nil {
		return nil, ""
	}
	if overview.TTFT.P95 != nil {
		return overview.TTFT.P95, "p95"
	}
	if overview.TTFT.P90 != nil {
		return overview.TTFT.P90, "p90"
	}
	if overview.TTFT.P99 != nil {
		return overview.TTFT.P99, "p99"
	}
	return nil, ""
}

func determineHealthPrimarySignal(overview *OpsDashboardOverview, meta *OpsHealthScoreMeta) string {
	if overview == nil || meta == nil {
		return "unknown"
	}
	if overview.SystemMetrics != nil {
		if overview.SystemMetrics.DBOK != nil && !*overview.SystemMetrics.DBOK {
			meta.Reasons = append(meta.Reasons, "db_down")
			return "infra"
		}
		if overview.SystemMetrics.RedisOK != nil && !*overview.SystemMetrics.RedisOK {
			meta.Reasons = append(meta.Reasons, "redis_down")
			return "infra"
		}
		if overview.SystemMetrics.CPUUsagePercent != nil && *overview.SystemMetrics.CPUUsagePercent > 80 {
			meta.Reasons = append(meta.Reasons, "cpu_high")
			return "infra"
		}
		if overview.SystemMetrics.MemoryUsagePercent != nil && *overview.SystemMetrics.MemoryUsagePercent > 85 {
			meta.Reasons = append(meta.Reasons, "memory_high")
			return "infra"
		}
	}
	if overview.UpstreamErrorRate > 0.02 {
		meta.Reasons = append(meta.Reasons, "upstream_error_rate")
		return "upstream"
	}
	if overview.ErrorRate > 0.005 {
		meta.Reasons = append(meta.Reasons, "error_rate")
		return "error"
	}
	if meta.TTFTReferenceMs != nil && *meta.TTFTReferenceMs > 1500 {
		meta.Reasons = append(meta.Reasons, "ttft")
		return "upstream_experience"
	}
	if meta.BusinessScore < 90 {
		return "business"
	}
	if meta.InfraScore < 90 {
		return "infra"
	}
	return "healthy"
}

// computeInfraHealth calculates infrastructure health score (0-100)
// Components: Storage (40%) + Compute Resources (30%) + Background Jobs (30%)
func computeInfraHealth(now time.Time, overview *OpsDashboardOverview) float64 {
	// Storage score: DB critical, Redis less critical
	storageScore := 100.0
	if overview.SystemMetrics != nil {
		if overview.SystemMetrics.DBOK != nil && !*overview.SystemMetrics.DBOK {
			storageScore = 0 // DB failure is critical
		} else if overview.SystemMetrics.RedisOK != nil && !*overview.SystemMetrics.RedisOK {
			storageScore = 50 // Redis failure is degraded but not critical
		}
	}

	// Compute resources score: CPU + Memory
	computeScore := 100.0
	if overview.SystemMetrics != nil {
		cpuScore := 100.0
		if overview.SystemMetrics.CPUUsagePercent != nil {
			cpuPct := clampFloat64(*overview.SystemMetrics.CPUUsagePercent, 0, 100)
			if cpuPct > 80 {
				if cpuPct <= 100 {
					cpuScore = (100 - cpuPct) / 20 * 100
				} else {
					cpuScore = 0
				}
			}
		}

		memScore := 100.0
		if overview.SystemMetrics.MemoryUsagePercent != nil {
			memPct := clampFloat64(*overview.SystemMetrics.MemoryUsagePercent, 0, 100)
			if memPct > 85 {
				if memPct <= 100 {
					memScore = (100 - memPct) / 15 * 100
				} else {
					memScore = 0
				}
			}
		}

		computeScore = (cpuScore + memScore) / 2
	}

	// Background jobs score
	jobScore := 100.0
	failedJobs := 0
	totalJobs := 0
	for _, hb := range overview.JobHeartbeats {
		if hb == nil {
			continue
		}
		totalJobs++
		if hb.LastErrorAt != nil && (hb.LastSuccessAt == nil || hb.LastErrorAt.After(*hb.LastSuccessAt)) {
			failedJobs++
		} else if hb.LastSuccessAt != nil && now.Sub(*hb.LastSuccessAt) > 15*time.Minute {
			failedJobs++
		}
	}
	if totalJobs > 0 && failedJobs > 0 {
		jobScore = (1 - float64(failedJobs)/float64(totalJobs)) * 100
	}

	// Weighted combination
	return storageScore*0.4 + computeScore*0.3 + jobScore*0.3
}

func clampFloat64(v float64, min float64, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
