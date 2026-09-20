package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type autoFailoverRepoStub struct {
	ProxyRepository
	proxies  []Proxy
	switches int
	restores int
}

func (r *autoFailoverRepoStub) ListAllForFallback(context.Context) ([]Proxy, error) {
	return r.proxies, nil
}
func (r *autoFailoverRepoStub) SwitchAccountsToBackup(context.Context, int64, int64) (int64, error) {
	r.switches++
	return 2, nil
}
func (r *autoFailoverRepoStub) RestoreAccountsFromBackup(context.Context, int64, int64) (int64, error) {
	r.restores++
	return 2, nil
}

type autoFailoverProberStub struct {
	mu           sync.Mutex
	failHost     map[string]int
	calls        map[string]int
	targetStatus int
}

func (p *autoFailoverProberStub) ProbeProxy(_ context.Context, proxyURL string) (*ProxyExitInfo, int64, error) {
	u, _ := url.Parse(proxyURL)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls[u.Hostname()]++
	if p.failHost[u.Hostname()] > 0 {
		p.failHost[u.Hostname()]--
		return nil, 20, errors.New("connection reset")
	}
	return &ProxyExitInfo{IP: "203.0.113.10"}, 20, nil
}

func (p *autoFailoverProberStub) ProbeProxyTarget(context.Context, string, string) (int, int64, error) {
	if p.targetStatus != 0 {
		return p.targetStatus, 10, nil
	}
	return 401, 10, nil
}

type autoFailoverCacheStub struct {
	mu     sync.Mutex
	values map[int64]*ProxyLatencyInfo
}

func (c *autoFailoverCacheStub) GetProxyLatencies(_ context.Context, ids []int64) (map[int64]*ProxyLatencyInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[int64]*ProxyLatencyInfo)
	for _, id := range ids {
		if v := c.values[id]; v != nil {
			copy := *v
			out[id] = &copy
		}
	}
	return out, nil
}

func (c *autoFailoverCacheStub) SetProxyLatency(_ context.Context, id int64, info *ProxyLatencyInfo) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	copy := *info
	c.values[id] = &copy
	return nil
}

func TestProxyAutoFailoverSwitchesAfterConsecutiveHealthyBackupChecks(t *testing.T) {
	backupID := int64(2)
	repo := &autoFailoverRepoStub{proxies: []Proxy{
		{ID: 1, Name: "primary", Host: "primary.test", Protocol: "http", Port: 80, Status: StatusActive, FallbackMode: FallbackModeProxy, BackupProxyID: &backupID},
		{ID: 2, Name: "backup", Host: "backup.test", Protocol: "http", Port: 80, Status: StatusActive},
	}}
	prober := &autoFailoverProberStub{failHost: map[string]int{"primary.test": 3}, calls: map[string]int{}}
	cache := &autoFailoverCacheStub{values: map[int64]*ProxyLatencyInfo{}}
	svc := NewProxyAutoFailoverService(repo, prober, cache).WithTiming(time.Hour, 0).WithThresholds(3, 5)

	for i := 0; i < 3; i++ {
		svc.runOnce(context.Background())
	}

	require.Equal(t, 1, repo.switches)
	state := cache.values[1]
	require.Equal(t, "switched", state.AutoFailoverStatus)
	require.Equal(t, int64(2), *state.AutoFailoverTargetProxy)
	require.Contains(t, state.AutoFailoverLastReason, "连续 3 次")
}

func TestProxyAutoFailoverRestoresAfterConsecutiveRecoveryChecks(t *testing.T) {
	backupID := int64(2)
	repo := &autoFailoverRepoStub{proxies: []Proxy{
		{ID: 1, Name: "primary", Host: "primary.test", Protocol: "http", Port: 80, Status: StatusActive, FallbackMode: FallbackModeProxy, BackupProxyID: &backupID},
		{ID: 2, Name: "backup", Host: "backup.test", Protocol: "http", Port: 80, Status: StatusActive},
	}}
	prober := &autoFailoverProberStub{failHost: map[string]int{"primary.test": 3}, calls: map[string]int{}}
	cache := &autoFailoverCacheStub{values: map[int64]*ProxyLatencyInfo{}}
	svc := NewProxyAutoFailoverService(repo, prober, cache).WithTiming(time.Hour, 0).WithThresholds(3, 2)
	for i := 0; i < 3; i++ {
		svc.runOnce(context.Background())
	}
	for i := 0; i < 3; i++ {
		svc.runOnce(context.Background())
	}
	require.Equal(t, 1, repo.restores)
}

func TestProxyAutoFailoverRecoveryDelayPreventsFlipFlop(t *testing.T) {
	backupID := int64(2)
	repo := &autoFailoverRepoStub{proxies: []Proxy{
		{ID: 1, Host: "primary.test", Protocol: "http", Port: 80, Status: StatusActive, FallbackMode: FallbackModeProxy, BackupProxyID: &backupID},
		{ID: 2, Host: "backup.test", Protocol: "http", Port: 80, Status: StatusActive},
	}}
	prober := &autoFailoverProberStub{failHost: map[string]int{"primary.test": 3}, calls: map[string]int{}}
	svc := NewProxyAutoFailoverService(repo, prober, nil).WithTiming(time.Hour, time.Hour).WithThresholds(3, 2)
	for i := 0; i < 3; i++ {
		svc.runOnce(context.Background())
	}
	for i := 0; i < 2; i++ {
		svc.runOnce(context.Background())
	}
	require.Zero(t, repo.restores)
}

func TestProxyAutoFailoverDoesNotProbeWithoutExplicitBackupMode(t *testing.T) {
	backupID := int64(2)
	repo := &autoFailoverRepoStub{proxies: []Proxy{
		{ID: 1, Host: "primary.test", Protocol: "http", Port: 80, Status: StatusActive, FallbackMode: FallbackModeNone, BackupProxyID: &backupID},
		{ID: 2, Host: "backup.test", Protocol: "http", Port: 80, Status: StatusActive},
	}}
	prober := &autoFailoverProberStub{failHost: map[string]int{"primary.test": 10}, calls: map[string]int{}}
	svc := NewProxyAutoFailoverService(repo, prober, nil).WithTiming(time.Hour, 0)
	svc.runOnce(context.Background())
	prober.mu.Lock()
	defer prober.mu.Unlock()
	require.Zero(t, prober.calls["primary.test"])
	require.Zero(t, repo.switches)
}

func TestProxyAutoFailoverOpenAIStatusClassification(t *testing.T) {
	for _, tc := range []struct {
		status  int
		healthy bool
	}{
		{status: 200, healthy: true},
		{status: 401, healthy: true},
		{status: 429, healthy: true},
		{status: 403, healthy: false},
		{status: 404, healthy: false},
		{status: 502, healthy: false},
	} {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			require.Equal(t, tc.healthy, isHealthyOpenAIProbeStatus(tc.status))
		})
	}
}

func TestProxyAutoFailoverSkipsNestedBackupChain(t *testing.T) {
	backupID := int64(2)
	nestedBackupID := int64(3)
	repo := &autoFailoverRepoStub{proxies: []Proxy{
		{ID: 1, Host: "primary.test", Protocol: "http", Port: 80, Status: StatusActive, FallbackMode: FallbackModeProxy, BackupProxyID: &backupID},
		{ID: 2, Host: "backup.test", Protocol: "http", Port: 80, Status: StatusActive, FallbackMode: FallbackModeProxy, BackupProxyID: &nestedBackupID},
		{ID: 3, Host: "last.test", Protocol: "http", Port: 80, Status: StatusActive},
	}}
	prober := &autoFailoverProberStub{failHost: map[string]int{"primary.test": 3}, calls: map[string]int{}}
	svc := NewProxyAutoFailoverService(repo, prober, nil).WithTiming(time.Hour, 0)
	svc.runOnce(context.Background())
	require.Zero(t, repo.switches)
	prober.mu.Lock()
	defer prober.mu.Unlock()
	require.Zero(t, prober.calls["primary.test"])
}
