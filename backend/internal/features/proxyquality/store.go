package proxyquality

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/features/proxyopen"
	"github.com/lib/pq"
)

var (
	ErrInvalid        = errors.New("需要独立 OpenAI 账号、池内当前代理及 2–10 条已开启监测的有效非主备代理")
	ErrConflict       = errors.New("配置已变化，请刷新后重试")
	ErrNotFound       = errors.New("账号不存在")
	errObservationGap = errors.New("observation gap changed during decision")
)

type Config struct {
	Enabled  bool    `json:"enabled"`
	ProxyIDs []int64 `json:"proxy_ids"`
	Version  int64   `json:"version"`
}
type Settings struct {
	Config
	Current   *int64          `json:"current_proxy_id"`
	Reason    string          `json:"reason"`
	Evaluated *time.Time      `json:"last_evaluated_at"`
	Switched  *time.Time      `json:"last_switch_at"`
	Choices   []Choice        `json:"choices"`
	Events    []Event         `json:"events"`
	Samples   []SampleSummary `json:"samples"`
}
type Choice struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type Event struct {
	From   int64     `json:"from_proxy_id"`
	To     int64     `json:"to_proxy_id"`
	Reason string    `json:"reason"`
	At     time.Time `json:"created_at"`
}
type SampleSummary struct {
	ProxyID     int64  `json:"proxy_id"`
	Model       string `json:"model"`
	Transport   string `json:"transport"`
	Total       int64  `json:"total"`
	Successes   int64  `json:"successes"`
	FirstTokens int64  `json:"first_tokens"`
	HTTP5xx     int64  `json:"http_5xx"`
}
type gapGuard interface{ gapSnapshot(int64) (uint64, bool) }
type Store struct {
	DB    *sql.DB
	guard gapGuard
}

func validPool(ids []int64) bool {
	return len(ids) >= 2 && validIDs(ids)
}
func validIDs(ids []int64) bool {
	if len(ids) > 10 {
		return false
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
func (s *Store) Read(ctx context.Context, id int64) (*Settings, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	r := &Settings{Config: Config{ProxyIDs: []int64{}}, Choices: []Choice{}, Events: []Event{}, Samples: []SampleSummary{}}
	err = tx.QueryRowContext(ctx, `SELECT a.proxy_id,coalesce(c.enabled,false),coalesce(c.proxy_ids,'{}'),coalesce(c.version,0),coalesce(c.reason,'disabled'),c.last_evaluated_at,c.last_switch_at
 FROM accounts a LEFT JOIN account_proxy_quality_configs c ON c.account_id=a.id WHERE a.id=$1 AND a.deleted_at IS NULL`, id).
		Scan(&r.Current, &r.Enabled, pq.Array(&r.ProxyIDs), &r.Version, &r.Reason, &r.Evaluated, &r.Switched)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT p.id,p.name FROM proxies p JOIN proxy_openai_monitor_configs c ON c.proxy_id=p.id
 WHERE p.deleted_at IS NULL AND p.status='active' AND (p.expires_at IS NULL OR p.expires_at>now())
 AND p.protocol IN ('http','https','socks5','socks5h') AND p.fallback_mode='none' AND c.enabled ORDER BY p.id LIMIT 500`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p Choice
		if err = rows.Scan(&p.ID, &p.Name); err != nil {
			rows.Close()
			return nil, err
		}
		r.Choices = append(r.Choices, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT from_proxy_id,to_proxy_id,reason,created_at FROM account_proxy_quality_events WHERE account_id=$1 ORDER BY id DESC LIMIT 20`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e Event
		if err = rows.Scan(&e.From, &e.To, &e.Reason, &e.At); err != nil {
			return nil, err
		}
		r.Events = append(r.Events, e)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if len(r.ProxyIDs) > 0 {
		rows, err = tx.QueryContext(ctx, `SELECT proxy_id,model,transport,total,successes,first_tokens,http_5xx FROM (
 SELECT s.proxy_id,s.model,s.transport,count(*) FILTER(WHERE NOT s.http_5xx) AS total,
 count(*) FILTER(WHERE s.success) AS successes,count(s.first_token_ms) FILTER(WHERE s.success) AS first_tokens,
 count(*) FILTER(WHERE s.http_5xx) AS http_5xx,
 row_number() OVER(PARTITION BY s.proxy_id ORDER BY max(s.observed_at) DESC,s.model,s.transport) AS rank
 FROM account_proxy_quality_observations s JOIN proxies p ON p.id=s.proxy_id
 WHERE s.account_id=$1 AND s.proxy_id=ANY($2) AND s.observed_at>=now()-interval '1 hour' AND s.observed_at<now()
 AND s.fingerprint=encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex')
 GROUP BY s.proxy_id,s.model,s.transport) ranked WHERE rank<=5 ORDER BY proxy_id,rank`, id, pq.Array(r.ProxyIDs))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var sample SampleSummary
			if err = rows.Scan(&sample.ProxyID, &sample.Model, &sample.Transport, &sample.Total, &sample.Successes, &sample.FirstTokens, &sample.HTTP5xx); err != nil {
				rows.Close()
				return nil, err
			}
			r.Samples = append(r.Samples, sample)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return r, tx.Commit()
}

type account struct {
	Proxy, Origin, Parent *int64
	Platform, Status      string
}

func lockAccount(ctx context.Context, tx *sql.Tx, id int64) (account, error) {
	var a account
	err := tx.QueryRowContext(ctx, `SELECT proxy_id,proxy_fallback_origin_id,parent_account_id,platform,status FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).
		Scan(&a.Proxy, &a.Origin, &a.Parent, &a.Platform, &a.Status)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return a, err
}

// Both configuration and worker use config -> account -> ordered proxies ->
// ordered monitor configs, serializing disable/manual edits and routing commits.
func lockPool(ctx context.Context, tx *sql.Tx, ids []int64) (bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,fallback_mode FROM proxies WHERE id=ANY($1) ORDER BY id FOR SHARE`, pq.Array(ids))
	if err != nil {
		return false, err
	}
	count := 0
	ok := true
	for rows.Next() {
		var id int64
		var mode string
		if err = rows.Scan(&id, &mode); err != nil {
			rows.Close()
			return false, err
		}
		count++
		ok = ok && mode == "none"
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT proxy_id FROM proxy_openai_monitor_configs WHERE proxy_id=ANY($1) ORDER BY proxy_id FOR SHARE`, pq.Array(ids))
	if err != nil {
		return false, err
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return false, err
		}
	}
	err = rows.Err()
	rows.Close()
	return ok && count == len(ids), err
}
func (s *Store) Configure(ctx context.Context, id int64, c Config) error {
	if c.Version < 0 || !validIDs(c.ProxyIDs) || (c.Enabled && !validPool(c.ProxyIDs)) {
		return ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO account_proxy_quality_configs(account_id) VALUES($1) ON CONFLICT DO NOTHING`, id)
	if err != nil {
		return err
	}
	var version int64
	if err = tx.QueryRowContext(ctx, `SELECT version FROM account_proxy_quality_configs WHERE account_id=$1 FOR UPDATE`, id).Scan(&version); err != nil {
		return err
	}
	if version != c.Version {
		return ErrConflict
	}
	a, err := lockAccount(ctx, tx, id)
	if err != nil {
		return err
	}
	if c.Enabled {
		if a.Platform != "openai" || a.Status != "active" || a.Parent != nil || a.Origin != nil || a.Proxy == nil {
			return ErrInvalid
		}
		contains := false
		for _, p := range c.ProxyIDs {
			contains = contains || p == *a.Proxy
		}
		if !contains {
			return ErrInvalid
		}
		ok, poolErr := lockPool(ctx, tx, c.ProxyIDs)
		if poolErr != nil {
			return poolErr
		}
		if !ok {
			return ErrInvalid
		}
		var now time.Time
		if err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		qs, qualityErr := proxyopen.ReadQuality(ctx, tx, c.ProxyIDs, now)
		if qualityErr != nil {
			return qualityErr
		}
		for _, q := range qs {
			if !q.Eligible || !q.Enabled {
				return ErrInvalid
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE account_proxy_quality_configs SET enabled=true,proxy_ids=$2,expected_proxy_id=$3,version=version+1,next_at=now(),reason='waiting',updated_at=now() WHERE account_id=$1`, id, pq.Array(c.ProxyIDs), a.Proxy)
		if err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO account_proxy_quality_account_health(account_id,version,last_gap_at) VALUES($1,1,clock_timestamp()) ON CONFLICT(account_id) DO UPDATE SET version=account_proxy_quality_account_health.version+1,last_gap_at=excluded.last_gap_at`, id)
		}
	} else {
		// Disabling remains possible even if the account/pool is no longer eligible.
		_, err = tx.ExecContext(ctx, `UPDATE account_proxy_quality_configs SET enabled=false,proxy_ids=coalesce($2::bigint[],proxy_ids),version=version+1,reason='disabled',updated_at=now() WHERE account_id=$1`, id, pq.Array(c.ProxyIDs))
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Step processes one due account. SKIP LOCKED and next_at provide durable
// fairness across replicas; the transaction owns the decision through commit.
func (s *Store) Step(ctx context.Context) (processed bool, stepErr error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	var id int64
	var version int64
	defer func() {
		_ = tx.Rollback()
		if stepErr != nil && id > 0 {
			// A broken account must not starve every later due account. This
			// retry marker is separate from the rolled-back binding transaction;
			// version protects a concurrent administrator's newer configuration.
			retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()
			reason := "evaluation_error"
			if errors.Is(stepErr, errObservationGap) {
				reason = "collection_unavailable"
			}
			_, _ = s.DB.ExecContext(retryCtx, `UPDATE account_proxy_quality_configs SET next_at=now()+interval '30 seconds',reason=$3 WHERE account_id=$1 AND version=$2 AND enabled AND next_at<=now()`, id, version, reason)
		}
	}()
	var ids []int64
	var expected *int64
	var last *time.Time
	err = tx.QueryRowContext(ctx, `SELECT account_id,proxy_ids,expected_proxy_id,last_switch_at,version FROM account_proxy_quality_configs WHERE enabled AND next_at<=now() ORDER BY next_at,account_id LIMIT 1 FOR UPDATE SKIP LOCKED`).
		Scan(&id, pq.Array(&ids), &expected, &last, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	a, err := lockAccount(ctx, tx, id)
	reason := ""
	disable := false
	var target int64
	var gapGeneration uint64
	var hasGuard bool
	switch {
	case errors.Is(err, ErrNotFound):
		reason = "account_unavailable"
		disable = true
	case err != nil:
		return true, err
	case a.Platform != "openai" || a.Parent != nil || a.Origin != nil:
		reason = "conflict"
		disable = true
	case a.Proxy == nil || expected == nil || *a.Proxy != *expected:
		reason = "manual_override"
		disable = true
	case a.Status != "active":
		reason = "account_unavailable"
	default:
		if s.guard != nil {
			var pending bool
			gapGeneration, pending = s.guard.gapSnapshot(id)
			hasGuard = true
			if pending {
				reason = "collection_unavailable"
				break
			}
		}
		ok, e := lockPool(ctx, tx, ids)
		if e != nil {
			return true, e
		}
		if !ok {
			reason = "conflict"
			break
		}
		var now time.Time
		if err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return true, err
		}
		qs, e := proxyopen.ReadQuality(ctx, tx, ids, now)
		if e != nil {
			return true, e
		}
		actual, e := readBusiness(ctx, tx, id, *a.Proxy, ids, now, last)
		if e != nil {
			return true, e
		}
		target, reason = choose(*a.Proxy, qs, actual, now, last)
		if target != 0 {
			var gapVersion int64
			var gapAt time.Time
			e = tx.QueryRowContext(ctx, `SELECT version,last_gap_at FROM account_proxy_quality_account_health WHERE account_id=$1 FOR SHARE`, id).Scan(&gapVersion, &gapAt)
			if errors.Is(e, sql.ErrNoRows) || (e == nil && (gapVersion != actual.GapVersion || now.Sub(gapAt) < 5*time.Minute)) {
				return true, errObservationGap
			}
			if e != nil {
				return true, e
			}
			if err = switchBinding(ctx, tx, id, *a.Proxy, target, reason); err != nil {
				return true, err
			}
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE account_proxy_quality_configs SET enabled=enabled AND NOT $2,reason=$3,
 expected_proxy_id=CASE WHEN $4::bigint>0 THEN $4 ELSE expected_proxy_id END,
 last_switch_at=CASE WHEN $4::bigint>0 THEN now() ELSE last_switch_at END,
 last_evaluated_at=now(),next_at=now()+interval '30 seconds',updated_at=now(),version=version+CASE WHEN $2 THEN 1 ELSE 0 END WHERE account_id=$1`, id, disable, reason, target)
	if err != nil {
		return true, err
	}
	if target != 0 && hasGuard {
		versionNow, pending := s.guard.gapSnapshot(id)
		if pending || versionNow != gapGeneration {
			return true, errObservationGap
		}
	}
	return true, tx.Commit()
}
