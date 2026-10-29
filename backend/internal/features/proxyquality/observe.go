package proxyquality

import (
	"context"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type observation struct {
	accountID  int64
	proxy      service.Proxy
	model      string
	transport  string
	success    bool
	persistent bool
	http5xx    bool
	firstToken *int
	at         time.Time
}

type accountGap struct {
	generation atomic.Uint64
	flushed    atomic.Uint64
}

func (r *Runner) stateFor(id int64) *accountGap {
	value, _ := r.states.LoadOrStore(id, &accountGap{})
	return value.(*accountGap)
}
func (r *Runner) noteGap(id int64) { r.stateFor(id).generation.Add(1) }
func (r *Runner) gapSnapshot(id int64) (uint64, bool) {
	value, ok := r.states.Load(id)
	if !ok {
		return 0, true
	}
	state := value.(*accountGap)
	generation := state.generation.Load()
	return generation, generation != state.flushed.Load()
}

// ObserveOpenAIAccount is deliberately nonblocking on the gateway hot path.
// Only a copied proxy snapshot is queued; no request or credential is persisted.
func (r *Runner) ObserveOpenAIAccount(a *service.Account, model, transport string, success, persistent, http5xx bool, firstTokenMs *int) {
	if r == nil || a == nil || a.Platform != "openai" {
		return
	}
	owner := a.ID
	if a.ParentAccountID != nil {
		owner = *a.ParentAccountID
	}
	active, ok := r.active.Load().(map[int64]struct{})
	if !ok {
		return
	}
	if _, ok = active[owner]; !ok {
		return
	}
	if a.Proxy == nil || a.ProxyID == nil || *a.ProxyID != a.Proxy.ID {
		r.noteGap(owner)
		return
	}
	model = strings.TrimSpace(model)
	if model == "" || len(model) > 200 {
		r.noteGap(owner)
		return
	}
	var ttft *int
	if firstTokenMs != nil && *firstTokenMs >= 0 {
		n := *firstTokenMs
		ttft = &n
	}
	entry := observation{accountID: owner, proxy: *a.Proxy, model: model, transport: transport, success: success, persistent: persistent, http5xx: http5xx, firstToken: ttft, at: time.Now().UTC()}
	select {
	case r.observations <- entry:
	default:
		r.noteGap(owner)
	}
}

func (r *Runner) runObserver(ctx context.Context) {
	defer r.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	r.refreshObserver(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.refreshObserver(ctx)
		case event := <-r.observations:
			c, cancel := context.WithTimeout(ctx, time.Second)
			r.flushAccountGap(c, event.accountID)
			if err := r.store.recordObservation(c, event); err != nil {
				r.noteGap(event.accountID)
				if ctx.Err() == nil {
					log.Printf("[AccountProxyQuality] observation write failed: %v", err)
				}
			}
			cancel()
		}
	}
}
func (r *Runner) refreshObserver(ctx context.Context) {
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	r.flushPendingGaps(c)
	previous, _ := r.active.Load().(map[int64]struct{})
	rows, err := r.store.DB.QueryContext(c, `SELECT account_id FROM account_proxy_quality_configs WHERE enabled`)
	if err != nil {
		for id := range previous {
			r.noteGap(id)
		}
		r.active.Store(map[int64]struct{}{})
		return
	}
	active := map[int64]struct{}{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			break
		}
		active[id] = struct{}{}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		for id := range previous {
			r.noteGap(id)
		}
		r.active.Store(map[int64]struct{}{})
		return
	}
	for id := range active {
		if _, ok := previous[id]; !ok {
			r.noteGap(id)
		}
	}
	r.active.Store(active)
	r.flushPendingGaps(c)
}
func (r *Runner) flushAccountGap(ctx context.Context, id int64) {
	state := r.stateFor(id)
	generation := state.generation.Load()
	if generation == state.flushed.Load() {
		return
	}
	_, err := r.store.DB.ExecContext(ctx, `INSERT INTO account_proxy_quality_account_health(account_id,version,last_gap_at) VALUES($1,1,clock_timestamp()) ON CONFLICT(account_id) DO UPDATE SET version=account_proxy_quality_account_health.version+1,last_gap_at=excluded.last_gap_at`, id)
	if err == nil {
		state.flushed.Store(generation)
	}
}
func (r *Runner) flushPendingGaps(ctx context.Context) {
	r.states.Range(func(key, _ any) bool {
		if ctx.Err() != nil {
			return false
		}
		r.flushAccountGap(ctx, key.(int64))
		return true
	})
}
func (s *Store) recordObservation(ctx context.Context, e observation) error {
	p := e.proxy
	_, err := s.DB.ExecContext(ctx, `INSERT INTO account_proxy_quality_observations(account_id,proxy_id,fingerprint,model,success,first_token_ms,observed_at,transport,persistent_failure,http_5xx)
 SELECT $1,p.id,encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex'),$2,$3,$4,$5,$12,$13,$14
 FROM proxies p WHERE p.id=$6 AND p.deleted_at IS NULL AND p.protocol=$7 AND p.host=$8 AND p.port=$9
 AND coalesce(p.username,'')=$10 AND coalesce(p.password,'')=$11
 AND EXISTS(SELECT 1 FROM account_proxy_quality_configs c WHERE c.account_id=$1 AND c.enabled AND (c.all_eligible OR p.id=ANY(c.proxy_ids)))`,
		e.accountID, e.model, e.success, e.firstToken, e.at, p.ID, p.Protocol, p.Host, p.Port, p.Username, p.Password, e.transport, e.persistent, e.http5xx)
	return err
}
