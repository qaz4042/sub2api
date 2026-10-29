package proxyopen

import (
	"context"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type capacityProbe struct {
	delay                  atomic.Int64
	active, peak, overlaps atomic.Int32
	mu                     sync.Mutex
	running                map[string]bool
}

func (p *capacityProbe) ProbeProxy(context.Context, string) (*service.ProxyExitInfo, int64, error) {
	panic("unexpected health probe")
}
func (p *capacityProbe) ProbeProxyTarget(ctx context.Context, proxyURL, _ string) (int, int64, error) {
	u, _ := url.Parse(proxyURL)
	key := u.Host
	p.mu.Lock()
	if p.running[key] {
		p.overlaps.Add(1)
	}
	p.running[key] = true
	p.mu.Unlock()
	a := p.active.Add(1)
	defer func() { p.active.Add(-1); p.mu.Lock(); delete(p.running, key); p.mu.Unlock() }()
	for {
		peak := p.peak.Load()
		if a <= peak || p.peak.CompareAndSwap(peak, a) {
			break
		}
	}
	delay := time.Duration(p.delay.Load())
	select {
	case <-ctx.Done():
		return 0, 0, ctx.Err()
	case <-time.After(delay):
		return 401, delay.Milliseconds(), nil
	}
}
func TestTenProxyCapacity(t *testing.T) {
	if os.Getenv("PROXY_MONITOR_CAPACITY") != "1" {
		t.Skip("opt-in real-time 10 proxy / 10s capacity check")
	}
	s := testStore(t)
	ctx := context.Background()
	_, err := s.DB.Exec(`INSERT INTO proxies(id) SELECT generate_series(3,10); UPDATE proxies SET port=8000+id`)
	require.NoError(t, err)
	for i := int64(1); i <= 10; i++ {
		require.NoError(t, s.Configure(ctx, i, Config{Enabled: true, Interval: 10}))
	}
	p := &capacityProbe{running: map[string]bool{}}
	p.delay.Store(int64(20 * time.Millisecond))
	r := New(s, p)
	r.Start()
	defer r.Stop()
	time.Sleep(32 * time.Second)
	var normal int
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM proxy_openai_monitor_samples`).Scan(&normal))
	require.GreaterOrEqual(t, normal, 20)
	p.delay.Store(int64(10050 * time.Millisecond))
	slowStart := time.Now()
	time.Sleep(42 * time.Second)
	var slow int
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM proxy_openai_monitor_samples WHERE started_at>$1`, slowStart).Scan(&slow))
	require.GreaterOrEqual(t, slow, 10)
	p.delay.Store(int64(20 * time.Millisecond))
	recovery := time.Now()
	time.Sleep(25 * time.Second)
	var recovered int
	var deviation float64
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM proxy_openai_monitor_samples WHERE started_at>$1`, recovery).Scan(&recovered))
	require.GreaterOrEqual(t, recovered, 10)
	require.NoError(t, s.DB.QueryRow(`SELECT max(extract(epoch FROM (started_at-planned_at))) FROM proxy_openai_monitor_samples`).Scan(&deviation))
	require.LessOrEqual(t, deviation, 2.0)
	var skipped int
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM (SELECT extract(epoch FROM (planned_at-lag(planned_at) OVER(PARTITION BY proxy_id ORDER BY planned_at))) AS gap FROM proxy_openai_monitor_samples) s WHERE gap>11`).Scan(&skipped))
	require.Greater(t, skipped, 0)
	require.Zero(t, p.overlaps.Load())
	require.LessOrEqual(t, p.peak.Load(), int32(Workers))
	t.Logf("workers=%d; normal samples=%d; slow samples=%d; recovered samples=%d; skipped gaps=%d; max startup deviation=%.3fs; peak concurrency=%d; overlaps=%d", Workers, normal, slow, recovered, skipped, deviation, p.peak.Load(), p.overlaps.Load())
}
