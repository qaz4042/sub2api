package proxyquality

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("PROXY_QUALITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set PROXY_QUALITY_TEST_DSN to a disposable PostgreSQL database")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("quality_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close(); admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); admin.Close() })
	_, err = db.Exec(`CREATE TABLE proxies(id bigint PRIMARY KEY,name text NOT NULL DEFAULT 'proxy',protocol text NOT NULL DEFAULT 'http',host text NOT NULL DEFAULT 'localhost',port int NOT NULL DEFAULT 8080,username text,password text,status text NOT NULL DEFAULT 'active',expires_at timestamptz,deleted_at timestamptz,fallback_mode text NOT NULL DEFAULT 'none');
 CREATE TABLE accounts(id bigint PRIMARY KEY,proxy_id bigint,proxy_fallback_origin_id bigint,parent_account_id bigint,platform text NOT NULL DEFAULT 'openai',type text NOT NULL DEFAULT 'oauth',status text NOT NULL DEFAULT 'active',extra jsonb NOT NULL DEFAULT '{}',temp_unschedulable_until timestamptz,temp_unschedulable_reason text,updated_at timestamptz NOT NULL DEFAULT now(),deleted_at timestamptz);
 CREATE TABLE scheduler_outbox(id bigserial PRIMARY KEY,event_type text NOT NULL,account_id bigint,payload jsonb);
 INSERT INTO proxies(id) VALUES(1),(2),(3);INSERT INTO accounts(id,proxy_id) VALUES(1,1),(2,1);`)
	require.NoError(t, err)
	for _, name := range []string{"244_proxy_openai_monitor.sql", "250_account_proxy_quality.sql", "251_account_proxy_quality_observations.sql", "252_account_proxy_quality_safety.sql", "253_account_proxy_quality_scope.sql", "254_account_proxy_quality_monitor_confirmation.sql"} {
		b, e := os.ReadFile(filepath.Join("..", "..", "..", "migrations", name))
		require.NoError(t, e)
		_, e = db.Exec(string(b))
		require.NoError(t, e)
	}
	_, err = db.Exec(`INSERT INTO proxy_openai_monitor_configs(proxy_id,enabled,interval_seconds) SELECT id,true,30 FROM proxies`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO account_proxy_quality_collection_health VALUES(1,now()-interval '1 hour')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO account_proxy_quality_account_health(account_id,last_gap_at) VALUES(1,now()-interval '1 hour'),(2,now()-interval '1 hour')`)
	require.NoError(t, err)
	return &Store{DB: db}
}
func samples(t *testing.T, s *Store) {
	t.Helper()
	_, err := s.DB.Exec(`INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,latency_ms,http_status)
 SELECT p.id,encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex'),1,p.id||'-'||n,
 now()-n*interval '20 seconds',now()-n*interval '20 seconds',now()-n*interval '20 seconds',true,CASE WHEN p.id=1 THEN 1500 ELSE 300 END,401
 FROM proxies p CROSS JOIN generate_series(1,12) n`)
	require.NoError(t, err)
	_, err = s.DB.Exec(`INSERT INTO account_proxy_quality_observations(account_id,proxy_id,fingerprint,model,success,first_token_ms,observed_at,transport)
 SELECT a.id,p.id,encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex'), 'gpt',true,CASE WHEN p.id=1 THEN 2500 ELSE 500 END,now()-n*interval '20 seconds','http'
 FROM accounts a CROSS JOIN proxies p CROSS JOIN generate_series(1,10) n`)
	require.NoError(t, err)
}
func configured(t *testing.T, s *Store) {
	t.Helper()
	require.NoError(t, s.Configure(context.Background(), 1, Config{Enabled: true, ProxyIDs: []int64{1, 2}}))
	_, err := s.DB.Exec(`UPDATE account_proxy_quality_account_health SET last_gap_at=now()-interval '1 hour' WHERE account_id=1`)
	require.NoError(t, err)
}
func addTransportFailure(t *testing.T, s *Store, accountID int64) {
	t.Helper()
	_, err := s.DB.Exec(`INSERT INTO account_proxy_quality_observations(account_id,proxy_id,fingerprint,model,transport,success,persistent_failure,observed_at)
 SELECT $1,p.id,encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex'),'gpt','http',false,true,clock_timestamp() FROM accounts a JOIN proxies p ON p.id=a.proxy_id WHERE a.id=$1`, accountID)
	require.NoError(t, err)
}
func binding(t *testing.T, s *Store) int64 {
	t.Helper()
	var id int64
	require.NoError(t, s.DB.QueryRow(`SELECT proxy_id FROM accounts WHERE id=1`).Scan(&id))
	return id
}
func evaluate(t *testing.T, s *Store) {
	t.Helper()
	more, err := s.Step(context.Background())
	require.NoError(t, err)
	require.True(t, more)
}
func due(t *testing.T, s *Store) {
	t.Helper()
	_, err := s.DB.Exec(`UPDATE account_proxy_quality_configs SET next_at=now()-interval '1 second' WHERE account_id=1`)
	require.NoError(t, err)
}

func TestDefaultAndConfigurationValidation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	r, err := s.Read(ctx, 1)
	require.NoError(t, err)
	require.False(t, r.Enabled)
	require.Empty(t, r.Events)
	require.Len(t, r.Choices, 3)
	more, err := s.Step(ctx)
	require.NoError(t, err)
	require.False(t, more)
	require.EqualValues(t, 1, binding(t, s))
	for _, ids := range [][]int64{nil, {1}, {1, 1}, {2, 3}, {1, -1}, {1, 9}} {
		require.ErrorIs(t, s.Configure(ctx, 1, Config{Enabled: true, ProxyIDs: ids}), ErrInvalid)
	}
	configured(t, s)
	require.ErrorIs(t, s.Configure(ctx, 1, Config{Enabled: false, Version: 0}), ErrConflict)
	_, err = s.DB.Exec(`UPDATE proxies SET status='inactive';UPDATE accounts SET status='error' WHERE id=1`)
	require.NoError(t, err)
	require.NoError(t, s.Configure(ctx, 1, Config{Enabled: false, Version: 1}))
	require.EqualValues(t, 1, binding(t, s))
}

func TestDisabledConfigurationCanSavePoolWithoutTakingOver(t *testing.T) {
	s := testStore(t)
	require.NoError(t, s.Configure(context.Background(), 1, Config{Enabled: false, ProxyIDs: []int64{1, 2}}))
	r, err := s.Read(context.Background(), 1)
	require.NoError(t, err)
	require.False(t, r.Enabled)
	require.Equal(t, []int64{1, 2}, r.ProxyIDs)
	require.EqualValues(t, 1, binding(t, s))
}
func TestSwitchAtomicAuditOutboxCooldownAndDisable(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	samples(t, s)
	configured(t, s)
	addTransportFailure(t, s, 1)
	evaluate(t, s)
	require.EqualValues(t, 2, binding(t, s))
	r, err := s.Read(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "business_degraded", r.Reason)
	require.Len(t, r.Events, 1)
	require.NotNil(t, r.Switched)
	var kind string
	var id int64
	require.NoError(t, s.DB.QueryRow(`SELECT event_type,account_id FROM scheduler_outbox`).Scan(&kind, &id))
	require.Equal(t, service.SchedulerOutboxEventAccountChanged, kind)
	require.EqualValues(t, 1, id)
	// Another healthy account on the same source is never swept along.
	require.NoError(t, s.DB.QueryRow(`SELECT proxy_id FROM accounts WHERE id=2`).Scan(&id))
	require.EqualValues(t, 1, id)
	due(t, s)
	evaluate(t, s)
	r, err = s.Read(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "cooldown", r.Reason)
	require.Len(t, r.Events, 1)
	require.NoError(t, s.Configure(ctx, 1, Config{Enabled: false, Version: r.Version}))
	due(t, s)
	more, err := s.Step(ctx)
	require.NoError(t, err)
	require.False(t, more)
	require.EqualValues(t, 2, binding(t, s))
}
func TestQualityIgnoresOldFingerprintAndUnknownData(t *testing.T) {
	for _, change := range []string{"connection", "stale", "paused", "failures", "short", "deleted", "fixed_failover"} {
		t.Run(change, func(t *testing.T) {
			s := testStore(t)
			samples(t, s)
			configured(t, s)
			queries := map[string]string{
				"connection":     `UPDATE proxies SET password='new' WHERE id=2`,
				"stale":          `UPDATE proxy_openai_monitor_samples SET started_at=started_at-interval '10 minutes',finished_at=finished_at-interval '10 minutes' WHERE proxy_id=2`,
				"paused":         `UPDATE proxy_openai_monitor_configs SET enabled=false WHERE proxy_id=2`,
				"failures":       `UPDATE proxy_openai_monitor_samples SET success=false,latency_ms=NULL WHERE proxy_id=2`,
				"short":          `DELETE FROM proxy_openai_monitor_samples WHERE proxy_id=2 AND started_at<now()-interval '90 seconds'`,
				"deleted":        `UPDATE proxies SET deleted_at=now() WHERE id=2`,
				"fixed_failover": `UPDATE proxies SET fallback_mode='proxy' WHERE id=2`,
			}
			_, err := s.DB.Exec(queries[change])
			require.NoError(t, err)
			evaluate(t, s)
			require.EqualValues(t, 1, binding(t, s))
			r, err := s.Read(context.Background(), 1)
			require.NoError(t, err)
			require.Empty(t, r.Events)
		})
	}
}
func TestManualOverrideAndModeConflict(t *testing.T) {
	for _, change := range []string{"proxy", "direct", "origin", "platform", "shadow"} {
		t.Run(change, func(t *testing.T) {
			s := testStore(t)
			samples(t, s)
			configured(t, s)
			changes := map[string]string{"proxy": "proxy_id=3", "direct": "proxy_id=NULL", "origin": "proxy_fallback_origin_id=3", "platform": "platform='anthropic'", "shadow": "parent_account_id=2"}
			_, err := s.DB.Exec(`UPDATE accounts SET ` + changes[change] + ` WHERE id=1`)
			require.NoError(t, err)
			evaluate(t, s)
			r, err := s.Read(context.Background(), 1)
			require.NoError(t, err)
			require.False(t, r.Enabled)
			require.Empty(t, r.Events)
			if change == "proxy" || change == "direct" {
				require.Equal(t, "manual_override", r.Reason)
			} else {
				require.Equal(t, "conflict", r.Reason)
			}
		})
	}
}
func TestRollbackOnOutboxFailure(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	addTransportFailure(t, s, 1)
	_, err := s.DB.Exec(`ALTER TABLE scheduler_outbox ADD CONSTRAINT reject_outbox CHECK(false)`)
	require.NoError(t, err)
	_, err = s.Step(context.Background())
	require.Error(t, err)
	require.EqualValues(t, 1, binding(t, s))
	r, err := s.Read(context.Background(), 1)
	require.NoError(t, err)
	require.Empty(t, r.Events)
	require.Nil(t, r.Switched)
	_, err = s.DB.Exec(`ALTER TABLE scheduler_outbox DROP CONSTRAINT reject_outbox`)
	require.NoError(t, err)
	due(t, s)
	evaluate(t, s)
	require.EqualValues(t, 2, binding(t, s))
}

func TestFailedAccountDoesNotStarveOtherAccounts(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	require.NoError(t, s.Configure(context.Background(), 2, Config{Enabled: true, ProxyIDs: []int64{1, 2}}))
	addTransportFailure(t, s, 1)
	addTransportFailure(t, s, 2)
	_, err := s.DB.Exec(`UPDATE account_proxy_quality_account_health SET last_gap_at=now()-interval '1 hour' WHERE account_id=2`)
	require.NoError(t, err)
	_, err = s.DB.Exec(`ALTER TABLE scheduler_outbox ADD CONSTRAINT reject_first CHECK(account_id<>1)`)
	require.NoError(t, err)
	_, err = s.Step(context.Background())
	require.Error(t, err)
	evaluate(t, s)
	var id int64
	require.NoError(t, s.DB.QueryRow(`SELECT proxy_id FROM accounts WHERE id=2`).Scan(&id))
	require.EqualValues(t, 2, id)
	r, err := s.Read(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "evaluation_error", r.Reason)
	require.Empty(t, r.Events)
}

func TestManualWriteWinsWhileWorkerWaitsForAccountLock(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	tx, err := s.DB.Begin()
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE accounts SET proxy_id=3 WHERE id=1`)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, e := s.Step(context.Background()); done <- e }()
	require.NoError(t, tx.Commit())
	require.NoError(t, <-done)
	r, err := s.Read(context.Background(), 1)
	require.NoError(t, err)
	require.False(t, r.Enabled)
	require.EqualValues(t, 3, *r.Current)
	require.Empty(t, r.Events)
}

func TestRouteDependentProbeInvalidation(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	addTransportFailure(t, s, 1)
	_, err := s.DB.Exec(`UPDATE accounts SET type='apikey',extra='{"upstream_billing_probe":{"status":"ok"},"ollama_cloud_usage_snapshot":{},"opencode_go_usage_snapshot":{},"unrelated":true}'::jsonb WHERE id=1`)
	require.NoError(t, err)
	evaluate(t, s)
	var probe, unrelated bool
	require.NoError(t, s.DB.QueryRow(`SELECT extra ?| array['upstream_billing_probe','ollama_cloud_usage_snapshot','opencode_go_usage_snapshot'],extra ? 'unrelated' FROM accounts WHERE id=1`).Scan(&probe, &unrelated))
	require.False(t, probe)
	require.True(t, unrelated)
}

func TestShadowProxyInheritanceAndRollback(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	addTransportFailure(t, s, 1)
	_, err := s.DB.Exec(`UPDATE accounts SET parent_account_id=1 WHERE id=2;ALTER TABLE scheduler_outbox ADD CONSTRAINT reject_shadow CHECK(account_id<>2)`)
	require.NoError(t, err)
	_, err = s.Step(context.Background())
	require.Error(t, err)
	require.EqualValues(t, 1, binding(t, s))
	var proxy int64
	require.NoError(t, s.DB.QueryRow(`SELECT proxy_id FROM accounts WHERE id=2`).Scan(&proxy))
	require.EqualValues(t, 1, proxy)
	_, err = s.DB.Exec(`ALTER TABLE scheduler_outbox DROP CONSTRAINT reject_shadow`)
	require.NoError(t, err)
	due(t, s)
	evaluate(t, s)
	require.NoError(t, s.DB.QueryRow(`SELECT proxy_id FROM accounts WHERE id=2`).Scan(&proxy))
	require.EqualValues(t, 2, proxy)
	var count int
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM scheduler_outbox`).Scan(&count))
	require.Equal(t, 2, count)
}

func TestActualSamplesRequireMatchingModelAndCurrentConnection(t *testing.T) {
	for _, change := range []string{"candidate_missing", "different_model", "different_transport", "old_fingerprint", "collection_gap"} {
		t.Run(change, func(t *testing.T) {
			s := testStore(t)
			samples(t, s)
			configured(t, s)
			queries := map[string]string{
				"candidate_missing":   `DELETE FROM account_proxy_quality_observations WHERE proxy_id=2`,
				"different_model":     `UPDATE account_proxy_quality_observations SET model='different' WHERE proxy_id=2`,
				"different_transport": `UPDATE account_proxy_quality_observations SET transport='ws' WHERE proxy_id=2`,
				"old_fingerprint":     `UPDATE account_proxy_quality_observations SET fingerprint='old' WHERE proxy_id=2`,
				"collection_gap":      `UPDATE account_proxy_quality_account_health SET last_gap_at=now() WHERE account_id=1`,
			}
			_, err := s.DB.Exec(queries[change])
			require.NoError(t, err)
			evaluate(t, s)
			require.EqualValues(t, 1, binding(t, s))
			settings, err := s.Read(context.Background(), 1)
			require.NoError(t, err)
			require.Empty(t, settings.Events)
			if change == "collection_gap" {
				require.Equal(t, "collection_unavailable", settings.Reason)
			} else {
				require.Equal(t, "monitor_confirming", settings.Reason)
			}
		})
	}
}

func TestObserverMarksGapWhenQueueFullOrRouteUnknown(t *testing.T) {
	r := New(nil)
	r.observations = make(chan observation, 1)
	r.active.Store(map[int64]struct{}{1: {}})
	id := int64(1)
	proxy := service.Proxy{ID: 1}
	account := &service.Account{ID: 1, Platform: "openai", ProxyID: &id, Proxy: &proxy}
	r.ObserveOpenAIAccount(account, "gpt", "http", true, false, false, nil)
	r.ObserveOpenAIAccount(account, "gpt", "http", false, true, false, nil)
	_, pending := r.gapSnapshot(1)
	require.True(t, pending)
	r.stateFor(1).flushed.Store(r.stateFor(1).generation.Load())
	account.Proxy = nil
	r.ObserveOpenAIAccount(account, "gpt", "http", true, false, false, nil)
	_, pending = r.gapSnapshot(1)
	require.True(t, pending)
}

func TestObservedRequestHistoryDoesNotDriveOptimization(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	_, err := s.DB.Exec(`DELETE FROM account_proxy_quality_observations`)
	require.NoError(t, err)
	evaluate(t, s)
	require.EqualValues(t, 1, binding(t, s))
	r := New(s)
	r.active.Store(map[int64]struct{}{1: {}})
	r.stateFor(1)
	for _, id := range []int64{1, 2} {
		proxy := service.Proxy{ID: id, Protocol: "http", Host: "localhost", Port: 8080}
		account := &service.Account{ID: 1, Platform: "openai", ProxyID: &id, Proxy: &proxy}
		ttft := 2500
		if id == 2 {
			ttft = 500
		}
		for i := 0; i < 5; i++ {
			r.ObserveOpenAIAccount(account, "gpt", "http", true, false, false, &ttft)
			select {
			case event := <-r.observations:
				require.NoError(t, s.recordObservation(context.Background(), event))
			default:
				t.Fatal("observation not queued")
			}
		}
	}
	due(t, s)
	evaluate(t, s)
	require.EqualValues(t, 1, binding(t, s))
}

func TestPersistentFailureSwitchClearsOnlyTransportCooldown(t *testing.T) {
	for _, reason := range []string{"upstream transport error (proxy/network): connection refused", "manual or quota cooldown"} {
		t.Run(reason, func(t *testing.T) {
			s := testStore(t)
			samples(t, s)
			configured(t, s)
			_, err := s.DB.Exec(`UPDATE accounts SET temp_unschedulable_until=now()+interval '10 minutes',temp_unschedulable_reason=$1 WHERE id=1`, reason)
			require.NoError(t, err)
			_, err = s.DB.Exec(`INSERT INTO account_proxy_quality_observations(account_id,proxy_id,fingerprint,model,transport,success,persistent_failure,observed_at)
			 SELECT 1,p.id,encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex'),'gpt','http',false,true,now() FROM proxies p WHERE p.id=1`)
			require.NoError(t, err)
			evaluate(t, s)
			require.EqualValues(t, 2, binding(t, s))
			var remaining *string
			require.NoError(t, s.DB.QueryRow(`SELECT temp_unschedulable_reason FROM accounts WHERE id=1`).Scan(&remaining))
			if strings.HasPrefix(reason, "upstream transport error") {
				require.Nil(t, remaining)
			} else {
				require.NotNil(t, remaining)
				require.Equal(t, reason, *remaining)
			}
		})
	}
}

type changingGapGuard struct{ calls int }

func (g *changingGapGuard) gapSnapshot(_ int64) (uint64, bool) {
	g.calls++
	if g.calls > 1 {
		return 1, true
	}
	return 0, false
}

func TestLocalGapDuringDecisionRollsBackSwitch(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	addTransportFailure(t, s, 1)
	s.guard = &changingGapGuard{}
	_, err := s.Step(context.Background())
	require.ErrorIs(t, err, errObservationGap)
	require.EqualValues(t, 1, binding(t, s))
	r, err := s.Read(context.Background(), 1)
	require.NoError(t, err)
	require.Empty(t, r.Events)
	require.Equal(t, "collection_unavailable", r.Reason)
}

func TestDatabaseGapVersionChangesWhileDecisionWaits(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	addTransportFailure(t, s, 1)
	tx, err := s.DB.Begin()
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE account_proxy_quality_account_health SET version=version+1,last_gap_at=clock_timestamp() WHERE account_id=1`)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, e := s.Step(context.Background()); done <- e }()
	require.Eventually(t, func() bool {
		var n int
		e := s.DB.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%account_proxy_quality_account_health%' AND query LIKE '%FOR SHARE%'`).Scan(&n)
		return e == nil && n > 0
	}, 3*time.Second, 20*time.Millisecond)
	require.NoError(t, tx.Commit())
	require.ErrorIs(t, <-done, errObservationGap)
	require.EqualValues(t, 1, binding(t, s))
	r, err := s.Read(context.Background(), 1)
	require.NoError(t, err)
	require.Empty(t, r.Events)
	require.Equal(t, "collection_unavailable", r.Reason)
}

func TestGapOnlyPausesAffectedAccount(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	require.NoError(t, s.Configure(context.Background(), 2, Config{Enabled: true, ProxyIDs: []int64{1, 2}}))
	addTransportFailure(t, s, 2)
	_, err := s.DB.Exec(`UPDATE account_proxy_quality_account_health SET last_gap_at=CASE WHEN account_id=1 THEN now() ELSE now()-interval '1 hour' END`)
	require.NoError(t, err)
	evaluate(t, s)
	r, err := s.Read(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "collection_unavailable", r.Reason)
	evaluate(t, s)
	var proxy int64
	require.NoError(t, s.DB.QueryRow(`SELECT proxy_id FROM accounts WHERE id=2`).Scan(&proxy))
	require.EqualValues(t, 2, proxy)
}

func TestObservationCleanupCatchesUpInBoundedBatches(t *testing.T) {
	s := testStore(t)
	_, err := s.DB.Exec(`INSERT INTO account_proxy_quality_observations(account_id,proxy_id,fingerprint,model,success,observed_at)
	 SELECT 1,1,'x','gpt',true,now()-interval '2 days' FROM generate_series(1,12001);
	 INSERT INTO account_proxy_quality_observations(account_id,proxy_id,fingerprint,model,success,observed_at) VALUES(1,1,'x','gpt',true,now())`)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		more, err := s.cleanupBatch(context.Background())
		require.NoError(t, err)
		require.Equal(t, i < 2, more)
	}
	var n int
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM account_proxy_quality_observations`).Scan(&n))
	require.Equal(t, 1, n)
}

func TestHTTP5xxNeedsRecentSameCohortCandidateSuccess(t *testing.T) {
	for _, different := range []bool{false, true} {
		t.Run(fmt.Sprint(different), func(t *testing.T) {
			s := testStore(t)
			samples(t, s)
			configured(t, s)
			_, err := s.DB.Exec(`UPDATE account_proxy_quality_observations SET observed_at=now()-interval '3 minutes',first_token_ms=2500 WHERE proxy_id=1;
		 UPDATE account_proxy_quality_observations SET first_token_ms=2500 WHERE proxy_id=2;
		 INSERT INTO account_proxy_quality_observations(account_id,proxy_id,fingerprint,model,transport,success,http_5xx,observed_at)
		 SELECT 1,p.id,encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex'),'gpt','http',false,true,now()-n*interval '10 seconds'
		 FROM proxies p CROSS JOIN generate_series(3,5) n WHERE p.id=1`)
			require.NoError(t, err)
			if different {
				_, err = s.DB.Exec(`UPDATE account_proxy_quality_observations SET model='other' WHERE proxy_id=2`)
				require.NoError(t, err)
			}
			evaluate(t, s)
			r, err := s.Read(context.Background(), 1)
			require.NoError(t, err)
			if different {
				require.EqualValues(t, 1, binding(t, s))
				require.Empty(t, r.Events)
			} else {
				require.EqualValues(t, 2, binding(t, s))
				require.Equal(t, "business_5xx", r.Reason)
			}
			found := false
			for _, sample := range r.Samples {
				if sample.ProxyID == 1 && sample.Model == "gpt" {
					found = true
					require.EqualValues(t, 3, sample.HTTP5xx)
				}
			}
			require.True(t, found)
		})
	}
}

func TestObserverRecordsOnlyEnabledAccountAndExactProxySnapshot(t *testing.T) {
	s := testStore(t)
	configured(t, s)
	r := New(s)
	r.active.Store(map[int64]struct{}{1: {}})
	proxy := service.Proxy{ID: 1, Protocol: "http", Host: "localhost", Port: 8080}
	proxyID := int64(1)
	account := &service.Account{ID: 1, Platform: "openai", ProxyID: &proxyID, Proxy: &proxy}
	first := 1200
	r.ObserveOpenAIAccount(account, "gpt", "http", true, false, false, &first)
	select {
	case event := <-r.observations:
		require.NoError(t, s.recordObservation(context.Background(), event))
	default:
		t.Fatal("missing observation")
	}
	var count int
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM account_proxy_quality_observations`).Scan(&count))
	require.Equal(t, 1, count)
	proxy.Password = "changed"
	r.ObserveOpenAIAccount(account, "gpt", "http", false, true, false, nil)
	select {
	case event := <-r.observations:
		require.NoError(t, s.recordObservation(context.Background(), event))
	default:
		t.Fatal("missing observation")
	}
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM account_proxy_quality_observations`).Scan(&count))
	require.Equal(t, 1, count)
	r.active.Store(map[int64]struct{}{})
	r.ObserveOpenAIAccount(account, "gpt", "http", true, false, false, &first)
	require.Empty(t, r.observations)
}
func TestConcurrentWorkersOnlySwitchOnce(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	addTransportFailure(t, s, 1)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Step(context.Background()); require.NoError(t, err) }()
	}
	wg.Wait()
	r, err := s.Read(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, r.Events, 1)
	require.EqualValues(t, 2, binding(t, s))
}

func TestMonitorOptimizationUsesThreeFreshConfirmations(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	_, err := s.DB.Exec(`DELETE FROM account_proxy_quality_observations`)
	require.NoError(t, err)
	evaluate(t, s)
	var candidate, count int
	require.NoError(t, s.DB.QueryRow(`SELECT monitor_candidate_id,monitor_confirmations FROM account_proxy_quality_configs WHERE account_id=1`).Scan(&candidate, &count))
	require.Equal(t, 2, candidate)
	require.Equal(t, 1, count)
	due(t, s)
	evaluate(t, s)
	require.NoError(t, s.DB.QueryRow(`SELECT monitor_confirmations FROM account_proxy_quality_configs WHERE account_id=1`).Scan(&count))
	require.Equal(t, 1, count, "repeated probes do not count")
	for i, seconds := range []int{10, 5} {
		_, err = s.DB.Exec(`INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,latency_ms,http_status)
 SELECT p.id,encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex'),1,p.id||'-new-'||$1,
 now()-$2*interval '1 second',now()-$2*interval '1 second',now()-$2*interval '1 second',true,CASE WHEN p.id=1 THEN 1500 ELSE 300 END,401
 FROM proxies p WHERE p.id IN (1,2)`, i, seconds)
		require.NoError(t, err)
		due(t, s)
		evaluate(t, s)
		if i == 0 {
			require.EqualValues(t, 1, binding(t, s))
		}
	}
	require.EqualValues(t, 2, binding(t, s))
	r, err := s.Read(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "monitor_improved", r.Reason)
	require.Len(t, r.Events, 1)
	require.Equal(t, "monitor_improved", r.Events[0].Reason)
	require.NoError(t, s.DB.QueryRow(`SELECT monitor_confirmations FROM account_proxy_quality_configs WHERE account_id=1`).Scan(&count))
	require.Zero(t, count)
}

func TestHTTP5xxPausesMonitorOptimizationUntilRecovered(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	_, err := s.DB.Exec(`DELETE FROM account_proxy_quality_observations`)
	require.NoError(t, err)
	evaluate(t, s)
	var count int
	require.NoError(t, s.DB.QueryRow(`SELECT monitor_confirmations FROM account_proxy_quality_configs WHERE account_id=1`).Scan(&count))
	require.Equal(t, 1, count)
	_, err = s.DB.Exec(`INSERT INTO account_proxy_quality_observations(account_id,proxy_id,fingerprint,model,transport,success,http_5xx,observed_at)
 SELECT 1,p.id,encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex'),'gpt','http',false,true,now()-n*interval '1 second'
 FROM proxies p CROSS JOIN generate_series(1,3) n WHERE p.id=1`)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = s.DB.Exec(`INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,latency_ms,http_status)
 SELECT p.id,encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex'),1,p.id||'-5xx-'||$1,
 now()-$2*interval '1 second',now()-$2*interval '1 second',now()-$2*interval '1 second',true,CASE WHEN p.id=1 THEN 1500 ELSE 300 END,401 FROM proxies p WHERE p.id IN (1,2)`, i, 15-i*5)
		require.NoError(t, err)
		due(t, s)
		evaluate(t, s)
		r, e := s.Read(context.Background(), 1)
		require.NoError(t, e)
		require.Equal(t, "business_5xx_waiting", r.Reason)
		require.Empty(t, r.Events)
		require.EqualValues(t, 1, binding(t, s))
		require.NoError(t, s.DB.QueryRow(`SELECT monitor_confirmations FROM account_proxy_quality_configs WHERE account_id=1`).Scan(&count))
		require.Zero(t, count)
	}
	_, err = s.DB.Exec(`INSERT INTO account_proxy_quality_observations(account_id,proxy_id,fingerprint,model,transport,success,observed_at)
 SELECT 1,p.id,encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex'),'gpt','http',true,clock_timestamp() FROM proxies p WHERE id=1`)
	require.NoError(t, err)
	due(t, s)
	evaluate(t, s)
	r, err := s.Read(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "monitor_confirming", r.Reason)
	require.EqualValues(t, 1, binding(t, s))
}

func TestSavingConfigResetsMonitorConfirmation(t *testing.T) {
	s := testStore(t)
	samples(t, s)
	configured(t, s)
	evaluate(t, s)
	var count int
	require.NoError(t, s.DB.QueryRow(`SELECT monitor_confirmations FROM account_proxy_quality_configs WHERE account_id=1`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, s.Configure(context.Background(), 1, Config{Enabled: true, ProxyIDs: []int64{1, 2}, Version: 1}))
	require.NoError(t, s.DB.QueryRow(`SELECT monitor_confirmations FROM account_proxy_quality_configs WHERE account_id=1`).Scan(&count))
	require.Zero(t, count)
}
func TestPausedAndUnsupportedPoolsCannotEnable(t *testing.T) {
	for _, change := range []string{"paused", "expired", "fixed", "unsupported", "shadow", "origin"} {
		t.Run(change, func(t *testing.T) {
			s := testStore(t)
			queries := map[string]string{"paused": `UPDATE proxy_openai_monitor_configs SET enabled=false WHERE proxy_id=2`, "expired": `UPDATE proxies SET expires_at=now() WHERE id=2`, "fixed": `UPDATE proxies SET fallback_mode='proxy' WHERE id=1`, "unsupported": `UPDATE accounts SET platform='anthropic' WHERE id=1`, "shadow": `UPDATE accounts SET parent_account_id=2 WHERE id=1`, "origin": `UPDATE accounts SET proxy_fallback_origin_id=3 WHERE id=1`}
			_, err := s.DB.Exec(queries[change])
			require.NoError(t, err)
			require.ErrorIs(t, s.Configure(context.Background(), 1, Config{Enabled: true, ProxyIDs: []int64{1, 2}}), ErrInvalid)
		})
	}
}
func TestAdminRouteBoundaryAndBadInput(t *testing.T) {
	s := testStore(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/admin/proxies", func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			c.AbortWithStatus(401)
			return
		}
		if c.GetHeader("Authorization") != "admin" {
			c.AbortWithStatus(403)
			return
		}
	})
	New(s).Register(g)
	for _, tt := range []struct {
		method, path, auth, body string
		status                   int
	}{
		{"GET", "/1", "", "", 401}, {"GET", "/1", "user", "", 403}, {"PUT", "/1", "user", "{}", 403},
		{"GET", "/1", "admin", "", 200}, {"GET", "/no", "admin", "", 400}, {"GET", "/999", "admin", "", 404},
		{"PUT", "/1", "admin", `{"enabled":true,"proxy_ids":[1]}`, 400},
		{"PUT", "/1", "admin", `{"enabled":true,"proxy_ids":[1,2],"version":0}`, 200},
		{"PUT", "/1", "admin", `{"enabled":false,"version":0}`, 409},
	} {
		req := httptest.NewRequest(tt.method, "/admin/proxies/account-auto-switch"+tt.path, strings.NewReader(tt.body))
		req.Header.Set("Authorization", tt.auth)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, tt.status, w.Code, w.Body.String())
		require.NotContains(t, w.Body.String(), "password")
		require.NotContains(t, w.Body.String(), "fingerprint")
	}
}

func TestDynamicScopeAndLegacyCompatibility(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	r, err := s.Read(ctx, 1)
	require.NoError(t, err)
	require.True(t, r.AllEligible, "new accounts default to dynamic scope")
	configured(t, s)
	r, err = s.Read(ctx, 1)
	require.NoError(t, err)
	require.False(t, r.AllEligible, "legacy API writes retain explicit scope")
	require.NoError(t, s.Configure(ctx, 1, Config{Enabled: true, AllEligible: true, Version: r.Version}))
	// A later addition participates without rewriting the account configuration.
	_, err = s.DB.Exec(`INSERT INTO proxies(id) VALUES(4); INSERT INTO proxy_openai_monitor_configs(proxy_id,enabled,interval_seconds) VALUES(4,true,30);
 UPDATE proxies SET status='inactive' WHERE id=2;
 UPDATE proxies SET fallback_mode='primary' WHERE id=3;
 UPDATE account_proxy_quality_account_health SET last_gap_at=now()-interval '1 hour' WHERE account_id=1`)
	require.NoError(t, err)
	samples(t, s)
	addTransportFailure(t, s, 1)
	r, err = s.Read(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, r.CandidateCount)
	require.Empty(t, r.ProxyIDs, "dynamic membership is not a saved snapshot")
	evaluate(t, s)
	require.EqualValues(t, 4, binding(t, s))
	r, err = s.Read(ctx, 1)
	require.NoError(t, err)
	require.NoError(t, s.Configure(ctx, 1, Config{Enabled: false, AllEligible: true, Version: r.Version}))
	require.EqualValues(t, 4, binding(t, s), "disable preserves binding")
}

func TestDynamicScopeRecordsRealObservations(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	require.NoError(t, s.Configure(ctx, 1, Config{Enabled: true, AllEligible: true}))
	event := observation{accountID: 1, proxy: service.Proxy{ID: 1, Protocol: "http", Host: "localhost", Port: 8080}, model: "gpt", transport: "http", success: true, at: time.Now()}
	require.NoError(t, s.recordObservation(ctx, event))
	var count int
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM account_proxy_quality_observations`).Scan(&count))
	require.Equal(t, 1, count, "dynamic scope must collect even with an empty explicit pool")
	event.proxy.Password = "stale"
	require.NoError(t, s.recordObservation(ctx, event))
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM account_proxy_quality_observations`).Scan(&count))
	require.Equal(t, 1, count, "snapshot validation still applies")
	event.proxy.Password = ""
	require.NoError(t, s.Configure(ctx, 1, Config{Enabled: false, AllEligible: true, Version: 1}))
	require.NoError(t, s.recordObservation(ctx, event))
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM account_proxy_quality_observations`).Scan(&count))
	require.Equal(t, 1, count, "disabled accounts stop collecting")
}
