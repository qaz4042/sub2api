// Package proxyopen owns opt-in OpenAI connectivity monitoring. It never updates
// proxy health, account scheduling, usage or billing.
package proxyopen

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

const fingerprintSQL = `encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex')`
const eligibleSQL = `p.deleted_at IS NULL AND p.protocol IN ('http','https','socks5','socks5h') AND p.status='active' AND (p.expires_at IS NULL OR p.expires_at>now())`

var ErrInvalid = errors.New("invalid monitor configuration or proxy unavailable")

type Config struct {
	Enabled  bool   `json:"enabled"`
	Interval int    `json:"interval_seconds"`
	Version  int64  `json:"version"`
	State    string `json:"runtime_state"`
}
type Store struct{ DB *sql.DB }
type job struct {
	Proxy                  service.Proxy
	Fingerprint, Execution string
	Version                int64
	Planned                time.Time
	Interval               int
}

func validInterval(n int) bool { return n == 10 || n == 30 || n == 60 }
func nextPlan(plan, now time.Time, interval int) time.Time {
	d := time.Duration(interval) * time.Second
	if plan.After(now) {
		return plan
	}
	return plan.Add((now.Sub(plan)/d + 1) * d)
}

func (s *Store) Configure(ctx context.Context, id int64, c Config) error {
	if !validInterval(c.Interval) {
		return ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var eligible bool
	err = tx.QueryRowContext(ctx, `SELECT (`+eligibleSQL+`) FROM proxies p WHERE p.id=$1 AND p.deleted_at IS NULL FOR UPDATE`, id).Scan(&eligible)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && c.Enabled && !eligible) {
		return ErrInvalid
	}
	if err != nil {
		return err
	}
	// A new version invalidates in-flight jobs, but does not split connection history.
	_, err = tx.ExecContext(ctx, `INSERT INTO proxy_openai_monitor_configs(proxy_id,enabled,interval_seconds,next_at,runtime_state)
 VALUES($1,$2,$3,now()+($1::bigint % $3::integer)*interval '1 second',CASE WHEN $2 THEN 'waiting' ELSE 'paused' END)
 ON CONFLICT(proxy_id) DO UPDATE SET enabled=$2,interval_seconds=$3,version=proxy_openai_monitor_configs.version+1,
 next_at=excluded.next_at,runtime_state=excluded.runtime_state,updated_at=now()`, id, c.Enabled, c.Interval)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Claim locks both rows briefly; probes run outside the transaction. Advancing at
// claim time means a failed save cannot cause the same planned round to rerun.
// The second return value distinguishes schedule advancement from no due work.
func (s *Store) claim(ctx context.Context) (*job, bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	j := &job{}
	var lease sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT p.id,p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''),`+fingerprintSQL+`,c.version,c.next_at,c.interval_seconds,c.lease_until
 FROM proxy_openai_monitor_configs c JOIN proxies p ON p.id=c.proxy_id
 WHERE c.enabled AND `+eligibleSQL+` AND c.next_at<=now() AND (c.lease_until IS NULL OR c.lease_until<=now())
 ORDER BY c.next_at LIMIT 1 FOR UPDATE OF p,c SKIP LOCKED`).Scan(&j.Proxy.ID, &j.Proxy.Protocol, &j.Proxy.Host, &j.Proxy.Port, &j.Proxy.Username, &j.Proxy.Password, &j.Fingerprint, &j.Version, &j.Planned, &j.Interval, &lease)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var now time.Time
	if err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return nil, false, err
	}
	next := nextPlan(j.Planned, now, j.Interval)
	if lease.Valid {
		_, err = tx.ExecContext(ctx, `UPDATE proxy_openai_monitor_configs SET next_at=$2,lease_until=NULL,execution_id=NULL,runtime_state='lease_recovered',updated_at=now() WHERE proxy_id=$1`, j.Proxy.ID, next)
		if err != nil {
			return nil, false, err
		}
		return nil, true, tx.Commit()
	}
	// Drop overdue rounds instead of catching up after downtime or saturation.
	if now.Sub(j.Planned) > 2*time.Second {
		_, err = tx.ExecContext(ctx, `UPDATE proxy_openai_monitor_configs SET next_at=$2,runtime_state='skipped',updated_at=now() WHERE proxy_id=$1`, j.Proxy.ID, next)
		if err != nil {
			return nil, false, err
		}
		return nil, true, tx.Commit()
	}
	j.Execution = uuid.NewString()
	_, err = tx.ExecContext(ctx, `UPDATE proxy_openai_monitor_configs SET next_at=$2,lease_until=clock_timestamp()+interval '20 seconds',execution_id=$3,runtime_state='running',updated_at=now() WHERE proxy_id=$1`, j.Proxy.ID, next, j.Execution)
	if err != nil {
		return nil, false, err
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return j, false, nil
}

type Sample struct {
	ID       int64     `json:"id"`
	Started  time.Time `json:"started_at"`
	Finished time.Time `json:"finished_at"`
	Success  bool      `json:"success"`
	Latency  *int64    `json:"latency_ms"`
	Status   int       `json:"http_status"`
	Error    string    `json:"error_class"`
}

func (s *Store) finish(ctx context.Context, j *job, r Sample) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var valid bool
	err = tx.QueryRowContext(ctx, `SELECT coalesce(c.enabled AND c.version=$2 AND c.execution_id=$3 AND c.lease_until>clock_timestamp() AND `+eligibleSQL+` AND `+fingerprintSQL+`=$4,false)
 FROM proxies p JOIN proxy_openai_monitor_configs c ON c.proxy_id=p.id WHERE p.id=$1 FOR UPDATE OF p,c`, j.Proxy.ID, j.Version, j.Execution, j.Fingerprint).Scan(&valid)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !valid {
		return false, nil
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,latency_ms,http_status,error_class)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(execution_id) DO NOTHING`, j.Proxy.ID, j.Fingerprint, j.Version, j.Execution, j.Planned, r.Started, r.Finished, r.Success, r.Latency, r.Status, r.Error)
	if err != nil {
		return false, err
	}
	// The original phase is retained even when a slow request crosses a period.
	_, err = tx.ExecContext(ctx, `UPDATE proxy_openai_monitor_configs SET lease_until=NULL,execution_id=NULL,
 next_at=next_at+(greatest(0,floor(extract(epoch FROM (clock_timestamp()-next_at))/interval_seconds)+1))*interval_seconds*interval '1 second',
 runtime_state='waiting',updated_at=now() WHERE proxy_id=$1`, j.Proxy.ID)
	if err != nil {
		return false, err
	}
	err = tx.Commit()
	return err == nil, err
}
func (s *Store) cleanup(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM proxy_openai_monitor_samples WHERE id IN (
 SELECT s.id FROM proxy_openai_monitor_samples s WHERE s.finished_at<now()-interval '7 days'
 OR NOT EXISTS(SELECT 1 FROM proxies p WHERE p.id=s.proxy_id AND p.deleted_at IS NULL) LIMIT 5000)`)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `DELETE FROM proxy_openai_monitor_configs WHERE proxy_id IN (
 SELECT c.proxy_id FROM proxy_openai_monitor_configs c WHERE NOT EXISTS(SELECT 1 FROM proxies p WHERE p.id=c.proxy_id AND p.deleted_at IS NULL) LIMIT 1000)`)
	return err
}

// Settings is deliberately independent of sample aggregation so an administrator
// can pause monitoring even when history queries fail.
type Settings struct {
	Config   Config `json:"config"`
	Eligible bool   `json:"eligible"`
}

func (s *Store) ReadConfig(ctx context.Context, id int64) (*Settings, error) {
	settings := &Settings{}
	err := s.DB.QueryRowContext(ctx, `SELECT coalesce(c.enabled,false),coalesce(c.interval_seconds,30),coalesce(c.version,0),coalesce(c.runtime_state,'paused'),(`+eligibleSQL+`)
 FROM proxies p LEFT JOIN proxy_openai_monitor_configs c ON c.proxy_id=p.id WHERE p.id=$1 AND p.deleted_at IS NULL`, id).Scan(&settings.Config.Enabled, &settings.Config.Interval, &settings.Config.Version, &settings.Config.State, &settings.Eligible)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalid
	}
	if err != nil {
		return nil, err
	}
	if settings.Config.Enabled && !settings.Eligible {
		settings.Config.State = "unavailable"
	}
	return settings, nil
}
