package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ProxyTargetProber is optional. Implementations that provide it test both
// the proxy connection and the OpenAI route; older probe implementations are
// still accepted and fall back to the base connectivity check.
type ProxyTargetProber interface {
	ProbeProxyTarget(ctx context.Context, proxyURL, targetURL string) (statusCode int, latencyMs int64, err error)
}

const (
	defaultProxyAutoFailoverInterval      = 30 * time.Second
	defaultProxyAutoFailoverFailureLimit  = 3
	defaultProxyAutoFailoverRecoveryLimit = 5
	defaultProxyAutoFailoverRecoveryDelay = 10 * time.Minute
	proxyAutoFailoverTargetURL            = "https://api.openai.com/v1/models"
	proxyAutoFailoverProbeTimeout         = 15 * time.Second
)

type proxyAutoFailoverState struct {
	Failures      int
	Recoveries    int
	LastFailure   *time.Time
	LastSuccess   *time.Time
	LastSwitch    *time.Time
	LastReason    string
	TargetProxyID *int64
	Status        string
}

// ProxyAutoFailoverService periodically checks enabled primary proxies and
// moves their accounts to a healthy configured backup after consecutive
// failures. It deliberately owns only health observations and delegates the
// account update to the repository's atomic SQL methods.
type ProxyAutoFailoverService struct {
	proxyRepo     ProxyRepository
	prober        ProxyExitInfoProber
	cache         ProxyLatencyCache
	interval      time.Duration
	failureLimit  int
	recoveryLimit int
	recoveryDelay time.Duration
	stopCh        chan struct{}
	stopOnce      sync.Once
	wg            sync.WaitGroup
	mu            sync.Mutex
	states        map[int64]*proxyAutoFailoverState
}

func NewProxyAutoFailoverService(proxyRepo ProxyRepository, prober ProxyExitInfoProber, cache ProxyLatencyCache) *ProxyAutoFailoverService {
	return &ProxyAutoFailoverService{
		proxyRepo:     proxyRepo,
		prober:        prober,
		cache:         cache,
		interval:      defaultProxyAutoFailoverInterval,
		failureLimit:  defaultProxyAutoFailoverFailureLimit,
		recoveryLimit: defaultProxyAutoFailoverRecoveryLimit,
		recoveryDelay: defaultProxyAutoFailoverRecoveryDelay,
		stopCh:        make(chan struct{}),
		states:        make(map[int64]*proxyAutoFailoverState),
	}
}

// WithTiming is intended for focused tests and deployments that need to tune
// the worker without adding a second configuration hierarchy.
func (s *ProxyAutoFailoverService) WithTiming(interval, recoveryDelay time.Duration) *ProxyAutoFailoverService {
	if interval > 0 {
		s.interval = interval
	}
	if recoveryDelay >= 0 {
		s.recoveryDelay = recoveryDelay
	}
	return s
}

func (s *ProxyAutoFailoverService) WithThresholds(failures, recoveries int) *ProxyAutoFailoverService {
	if failures > 0 {
		s.failureLimit = failures
	}
	if recoveries > 0 {
		s.recoveryLimit = recoveries
	}
	return s
}

func (s *ProxyAutoFailoverService) Start() {
	if s == nil || s.proxyRepo == nil || s.prober == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(context.Background())
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runOnce(context.Background())
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *ProxyAutoFailoverService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *ProxyAutoFailoverService) runOnce(parent context.Context) {
	if s == nil || s.proxyRepo == nil || s.prober == nil {
		return
	}
	listCtx, cancel := context.WithTimeout(parent, 10*time.Second)
	proxies, err := s.proxyRepo.ListAllForFallback(listCtx)
	cancel()
	if err != nil {
		log.Printf("[ProxyAutoFailover] list proxies failed: %v", err)
		return
	}
	byID := make(map[int64]Proxy, len(proxies))
	for i := range proxies {
		byID[proxies[i].ID] = proxies[i]
	}
	for i := range proxies {
		proxy := proxies[i]
		if !proxy.IsActive() || proxy.IsExpired(time.Now()) || proxy.FallbackMode != FallbackModeProxy || proxy.BackupProxyID == nil {
			continue
		}
		backup, ok := byID[*proxy.BackupProxyID]
		if !ok || !backup.IsActive() || backup.IsExpired(time.Now()) {
			s.recordDisabled(parent, proxy.ID, "备用代理不存在、未启用或已过期")
			continue
		}
		// A nested fallback chain is still valid for expiry handling, but it is
		// deliberately excluded from network failover. Otherwise the worker
		// cannot identify which automatic move should be reverted later.
		if backup.FallbackMode == FallbackModeProxy {
			s.recordDisabled(parent, proxy.ID, "备用代理仍配置了备用代理；自动切换只支持单级主备")
			continue
		}
		checkCtx, checkCancel := context.WithTimeout(parent, 45*time.Second)
		s.checkProxy(checkCtx, proxy, backup)
		checkCancel()
	}
}

func (s *ProxyAutoFailoverService) checkProxy(parent context.Context, primary, backup Proxy) {
	now := time.Now()
	state := s.stateFor(primary.ID)
	if !s.prepareBackupState(parent, primary, backup, state) {
		return
	}
	ctx, cancel := context.WithTimeout(parent, proxyAutoFailoverProbeTimeout)
	_, _, err := s.probe(ctx, primary)
	cancel()
	if err == nil {
		s.handleSuccess(parent, primary, backup, state, now)
		return
	}
	s.handleFailure(parent, primary, backup, state, now, err)
}

func (s *ProxyAutoFailoverService) probe(ctx context.Context, proxy Proxy) (*ProxyExitInfo, int64, error) {
	exitInfo, latency, err := s.prober.ProbeProxy(ctx, proxy.URL())
	if err != nil {
		return nil, latency, err
	}
	if targetProber, ok := s.prober.(ProxyTargetProber); ok {
		status, targetLatency, targetErr := targetProber.ProbeProxyTarget(ctx, proxy.URL(), proxyAutoFailoverTargetURL)
		if targetErr != nil {
			return exitInfo, targetLatency, fmt.Errorf("OpenAI 探测失败: %w", targetErr)
		}
		// 401/403/404/429 all prove that the request reached the target. A
		// server-side 5xx or an invalid response is treated as unavailable.
		if !isHealthyOpenAIProbeStatus(status) {
			return exitInfo, targetLatency, fmt.Errorf("OpenAI 返回 HTTP %d", status)
		}
		if targetLatency > latency {
			latency = targetLatency
		}
	}
	return exitInfo, latency, nil
}

func isHealthyOpenAIProbeStatus(status int) bool {
	// A request without credentials normally receives 401. A 2xx response is
	// also healthy, while 429 proves the route is reachable but should not be
	// treated as a proxy failure. 403/404/redirects and 5xx are not accepted:
	// they indicate policy/route/server failure rather than mere auth state.
	return (status >= 200 && status < 300) || status == 401 || status == 429
}

func (s *ProxyAutoFailoverService) handleFailure(ctx context.Context, primary, backup Proxy, state *proxyAutoFailoverState, now time.Time, probeErr error) {
	s.mu.Lock()
	wasSwitched := state.Status == "switched" || state.Status == "recovering"
	state.Failures++
	state.Recoveries = 0
	state.LastFailure = timePtr(now)
	if !wasSwitched {
		state.Status = "degraded"
	}
	state.LastReason = probeErr.Error()
	failures := state.Failures
	s.mu.Unlock()
	if failures < s.failureLimit || wasSwitched {
		s.persistState(ctx, primary.ID, state)
		return
	}
	backupCtx, cancel := context.WithTimeout(ctx, proxyAutoFailoverProbeTimeout)
	_, _, backupErr := s.probe(backupCtx, backup)
	cancel()
	if backupErr != nil {
		s.mu.Lock()
		state.LastReason = fmt.Sprintf("主代理失败；备用代理也不可用: %v", backupErr)
		s.mu.Unlock()
		s.persistState(ctx, primary.ID, state)
		return
	}
	repo, ok := s.proxyRepo.(ProxyAutoFailoverRepository)
	if !ok {
		s.mu.Lock()
		state.LastReason = "自动切换仓储未启用"
		s.mu.Unlock()
		s.persistState(ctx, primary.ID, state)
		return
	}
	changed, switchErr := repo.SwitchAccountsToBackup(ctx, primary.ID, backup.ID)
	s.mu.Lock()
	if switchErr != nil {
		state.LastReason = fmt.Sprintf("切换账号失败: %v", switchErr)
	} else {
		state.Status = "switched"
		state.TargetProxyID = autoFailoverInt64Ptr(backup.ID)
		state.LastSwitch = timePtr(now)
		state.LastReason = fmt.Sprintf("连续 %d 次探测失败，已切换到 %s（影响 %d 个账号）", failures, backup.Name, changed)
	}
	s.mu.Unlock()
	s.persistState(ctx, primary.ID, state)
}

func (s *ProxyAutoFailoverService) handleSuccess(ctx context.Context, primary, backup Proxy, state *proxyAutoFailoverState, now time.Time) {
	s.mu.Lock()
	state.LastSuccess = timePtr(now)
	if state.Status == "switched" || state.Status == "recovering" {
		state.Recoveries++
		state.Status = "recovering"
	} else {
		state.Recoveries = 0
		state.Failures = 0
		state.Status = "healthy"
		state.LastReason = "主代理探测正常"
	}
	recoveries := state.Recoveries
	lastSwitch := state.LastSwitch
	s.mu.Unlock()
	if recoveries >= s.recoveryLimit && lastSwitch != nil && now.Sub(*lastSwitch) >= s.recoveryDelay {
		if repo, ok := s.proxyRepo.(ProxyAutoFailoverRepository); ok {
			changed, err := repo.RestoreAccountsFromBackup(ctx, primary.ID, backup.ID)
			s.mu.Lock()
			if err != nil {
				state.LastReason = fmt.Sprintf("主代理已恢复，但回切失败: %v", err)
			} else {
				state.Status = "healthy"
				state.Failures = 0
				state.Recoveries = 0
				state.TargetProxyID = nil
				state.LastReason = fmt.Sprintf("主代理连续 %d 次探测正常，已回切（影响 %d 个账号）", recoveries, changed)
			}
			s.mu.Unlock()
		}
	}
	s.persistState(ctx, primary.ID, state)
}

func (s *ProxyAutoFailoverService) prepareBackupState(ctx context.Context, primary, backup Proxy, state *proxyAutoFailoverState) bool {
	s.mu.Lock()
	oldTarget := state.TargetProxyID
	active := state.Status == "switched" || state.Status == "recovering"
	s.mu.Unlock()
	if !active || oldTarget == nil || *oldTarget == backup.ID {
		return true
	}
	repo, ok := s.proxyRepo.(ProxyAutoFailoverRepository)
	if !ok {
		s.mu.Lock()
		state.LastReason = "备用代理已变更，但自动切换仓储未启用"
		s.mu.Unlock()
		s.persistState(ctx, primary.ID, state)
		return false
	}
	if _, err := repo.RestoreAccountsFromBackup(ctx, primary.ID, *oldTarget); err != nil {
		s.mu.Lock()
		state.LastReason = fmt.Sprintf("备用代理已变更，清理旧切换状态失败: %v", err)
		s.mu.Unlock()
		s.persistState(ctx, primary.ID, state)
		return false
	}
	s.mu.Lock()
	state.Failures = 0
	state.Recoveries = 0
	state.LastSwitch = nil
	state.TargetProxyID = nil
	state.Status = "unknown"
	state.LastReason = fmt.Sprintf("备用代理已从 %d 变更为 %d，已清理旧切换状态", *oldTarget, backup.ID)
	s.mu.Unlock()
	s.persistState(ctx, primary.ID, state)
	return true
}

func (s *ProxyAutoFailoverService) recordDisabled(ctx context.Context, proxyID int64, reason string) {
	s.mu.Lock()
	state := s.stateForLocked(proxyID)
	state.Status = "disabled"
	state.LastReason = reason
	s.mu.Unlock()
	s.persistState(ctx, proxyID, state)
}

func (s *ProxyAutoFailoverService) stateFor(proxyID int64) *proxyAutoFailoverState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateForLocked(proxyID)
}

func (s *ProxyAutoFailoverService) stateForLocked(proxyID int64) *proxyAutoFailoverState {
	if state := s.states[proxyID]; state != nil {
		return state
	}
	state := &proxyAutoFailoverState{Status: "unknown"}
	if s.cache != nil {
		if values, err := s.cache.GetProxyLatencies(context.Background(), []int64{proxyID}); err == nil {
			if info := values[proxyID]; info != nil {
				state.Failures = info.AutoFailoverFailures
				state.Recoveries = info.AutoFailoverRecoveries
				state.LastFailure = info.AutoFailoverLastFailure
				state.LastSuccess = info.AutoFailoverLastSuccess
				state.LastSwitch = info.AutoFailoverLastSwitch
				state.LastReason = info.AutoFailoverLastReason
				state.TargetProxyID = info.AutoFailoverTargetProxy
				if info.AutoFailoverStatus != "" {
					state.Status = info.AutoFailoverStatus
				}
			}
		}
	}
	s.states[proxyID] = state
	return state
}

func (s *ProxyAutoFailoverService) persistState(ctx context.Context, proxyID int64, state *proxyAutoFailoverState) {
	if s.cache == nil || state == nil {
		return
	}
	values, err := s.cache.GetProxyLatencies(ctx, []int64{proxyID})
	if err != nil {
		return
	}
	info := values[proxyID]
	if info == nil {
		info = &ProxyLatencyInfo{}
	}
	info.AutoFailoverStatus = state.Status
	info.AutoFailoverFailures = state.Failures
	info.AutoFailoverRecoveries = state.Recoveries
	info.AutoFailoverLastFailure = state.LastFailure
	info.AutoFailoverLastSuccess = state.LastSuccess
	info.AutoFailoverLastSwitch = state.LastSwitch
	info.AutoFailoverLastReason = state.LastReason
	info.AutoFailoverTargetProxy = state.TargetProxyID
	info.UpdatedAt = time.Now()
	_ = s.cache.SetProxyLatency(ctx, proxyID, info)
}

func timePtr(v time.Time) *time.Time      { return &v }
func autoFailoverInt64Ptr(v int64) *int64 { return &v }
