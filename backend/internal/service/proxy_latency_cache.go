package service

import (
	"context"
	"time"
)

type ProxyLatencyInfo struct {
	Success          bool      `json:"success"`
	LatencyMs        *int64    `json:"latency_ms,omitempty"`
	Message          string    `json:"message,omitempty"`
	IPAddress        string    `json:"ip_address,omitempty"`
	Country          string    `json:"country,omitempty"`
	CountryCode      string    `json:"country_code,omitempty"`
	Region           string    `json:"region,omitempty"`
	City             string    `json:"city,omitempty"`
	QualityStatus    string    `json:"quality_status,omitempty"`
	QualityScore     *int      `json:"quality_score,omitempty"`
	QualityGrade     string    `json:"quality_grade,omitempty"`
	QualitySummary   string    `json:"quality_summary,omitempty"`
	QualityCheckedAt *int64    `json:"quality_checked_at,omitempty"`
	QualityCFRay     string    `json:"quality_cf_ray,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
	// AutoFailover fields are kept in the same shared cache entry as the
	// existing probe result so the admin list can explain an automatic switch
	// without adding a second read path.
	AutoFailoverStatus      string     `json:"auto_failover_status,omitempty"`
	AutoFailoverFailures    int        `json:"auto_failover_failures,omitempty"`
	AutoFailoverRecoveries  int        `json:"auto_failover_recoveries,omitempty"`
	AutoFailoverLastFailure *time.Time `json:"auto_failover_last_failure,omitempty"`
	AutoFailoverLastSuccess *time.Time `json:"auto_failover_last_success,omitempty"`
	AutoFailoverLastSwitch  *time.Time `json:"auto_failover_last_switch,omitempty"`
	AutoFailoverLastReason  string     `json:"auto_failover_last_reason,omitempty"`
	AutoFailoverTargetProxy *int64     `json:"auto_failover_target_proxy,omitempty"`
}

type ProxyLatencyCache interface {
	GetProxyLatencies(ctx context.Context, proxyIDs []int64) (map[int64]*ProxyLatencyInfo, error)
	SetProxyLatency(ctx context.Context, proxyID int64, info *ProxyLatencyInfo) error
}
