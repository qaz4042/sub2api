package proxyopen

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const Workers = 16

type Runner struct {
	store  *Store
	prober service.ProxyTargetProber
	cancel context.CancelFunc
	done   chan struct{}
}

func New(dbStore *Store, prober service.ProxyExitInfoProber) *Runner {
	target, _ := prober.(service.ProxyTargetProber)
	return &Runner{store: dbStore, prober: target}
}
func (r *Runner) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	go func() {
		defer close(r.done)
		var wg sync.WaitGroup
		defer wg.Wait()
		slots := make(chan struct{}, Workers)
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		clean := time.NewTicker(time.Minute)
		defer clean.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-clean.C:
				c, stop := context.WithTimeout(ctx, 3*time.Second)
				err := r.store.cleanup(c)
				stop()
				if err != nil && ctx.Err() == nil {
					log.Printf("[ProxyOpenAIMonitor] cleanup failed: %v", err)
				}
			case <-tick.C:
				if r.prober == nil {
					continue
				}
				// Bound database work as well as running probes; advancing one
				// stale schedule must not prevent other due work from being claimed.
				scanCtx, scanCancel := context.WithTimeout(ctx, time.Second)
				for scanned := 0; scanned < 64 && len(slots) < cap(slots) && scanCtx.Err() == nil; scanned++ {
					j, advanced, err := r.store.claim(scanCtx)
					if err != nil {
						if ctx.Err() == nil {
							log.Printf("[ProxyOpenAIMonitor] claim failed: %v", err)
						}
						break
					}
					if advanced {
						continue
					}
					if j == nil {
						break
					}
					slots <- struct{}{}
					wg.Add(1)
					go func() { defer wg.Done(); defer func() { <-slots }(); r.run(ctx, j) }()
				}
				scanCancel()
			}
		}
	}()
}
func (r *Runner) Stop() {
	if r != nil && r.cancel != nil {
		r.cancel()
		<-r.done
	}
}
func (r *Runner) run(ctx context.Context, j *job) {
	probeCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	sample := Sample{Started: time.Now().UTC()}
	status, latency, err := r.prober.ProbeProxyTarget(probeCtx, j.Proxy.URL(), "https://api.openai.com/v1/models")
	sample.Finished = time.Now().UTC()
	sample.Status = status
	sample.Success, sample.Error = classify(status, err)
	if sample.Success {
		sample.Latency = &latency
	}
	if ctx.Err() != nil {
		return
	}
	saveCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	defer stop()
	if _, err = r.store.finish(saveCtx, j, sample); err != nil {
		log.Printf("[ProxyOpenAIMonitor] save failed (round missing): %v", err)
	}
}
func classify(status int, err error) (bool, string) {
	if err != nil {
		var n net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &n) && n.Timeout()) {
			return false, "timeout"
		}
		return false, "connection"
	}
	switch {
	case status == 200 || status == 401:
		return true, ""
	case status == 403:
		return false, "http_403"
	case status == 429:
		return false, "http_429"
	case status >= 500:
		return false, "http_5xx"
	default:
		return false, "http_other"
	}
}
