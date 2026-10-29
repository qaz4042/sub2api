package proxyopen

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Each test uses an isolated schema on an explicitly supplied disposable database.
func testStore(t *testing.T) *Store {
	return testStoreWithData(t, true)
}
func testStoreWithData(t *testing.T, existing bool) *Store {
	t.Helper()
	dsn := os.Getenv("PROXY_MONITOR_TEST_DSN")
	if dsn == "" {
		t.Skip("set PROXY_MONITOR_TEST_DSN to a disposable PostgreSQL database")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("monitor_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close(); admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); admin.Close() })
	_, err = db.Exec(`CREATE TABLE proxies(id bigint PRIMARY KEY,protocol text NOT NULL DEFAULT 'http',host text NOT NULL DEFAULT 'localhost',port int NOT NULL DEFAULT 8080,username text,password text,status text NOT NULL DEFAULT 'active',expires_at timestamptz,deleted_at timestamptz)`)
	require.NoError(t, err)
	// Existing proxy data remains untouched and monitoring is opt-in.
	if existing {
		_, err = db.Exec(`INSERT INTO proxies(id) VALUES(1),(2)`)
		require.NoError(t, err)
	}
	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "244_proxy_openai_monitor.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM proxy_openai_monitor_configs`).Scan(&count))
	require.Zero(t, count)
	return &Store{DB: db}
}
func due(t *testing.T, s *Store, id int64) {
	t.Helper()
	_, err := s.DB.Exec(`UPDATE proxy_openai_monitor_configs SET next_at=clock_timestamp()-interval '100 milliseconds' WHERE proxy_id=$1`, id)
	require.NoError(t, err)
}
func enable(t *testing.T, s *Store, id int64) {
	t.Helper()
	require.NoError(t, s.Configure(context.Background(), id, Config{Enabled: true, Interval: 10}))
	due(t, s, id)
}
func claimOne(t *testing.T, s *Store) *job {
	t.Helper()
	j, _, err := s.claim(context.Background())
	require.NoError(t, err)
	require.NotNil(t, j)
	return j
}
func successSample() Sample {
	lat := int64(100)
	return Sample{Started: time.Now().UTC(), Finished: time.Now().UTC(), Success: true, Latency: &lat, Status: 401}
}

func TestLeaseCompetitionRecoveryAndStaleWrites(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	enable(t, s, 1)
	var wins atomic.Int32
	var winner *job
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, _, err := s.claim(ctx)
			require.NoError(t, err)
			if j != nil {
				wins.Add(1)
				mu.Lock()
				winner = j
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, wins.Load())
	ok, err := s.finish(ctx, winner, successSample())
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = s.finish(ctx, winner, successSample())
	require.NoError(t, err)
	require.False(t, ok)
	due(t, s, 1)
	old := claimOne(t, s)
	_, err = s.DB.Exec(`UPDATE proxy_openai_monitor_configs SET lease_until=now()-interval '1 second',next_at=now()-interval '30 seconds'`)
	require.NoError(t, err)
	j, _, err := s.claim(ctx)
	require.NoError(t, err)
	require.Nil(t, j)
	ok, err = s.finish(ctx, old, successSample())
	require.NoError(t, err)
	require.False(t, ok)
	j, _, err = s.claim(ctx)
	require.NoError(t, err)
	require.Nil(t, j)
	due(t, s, 1)
	fresh := claimOne(t, s)
	require.NotEqual(t, old.Execution, fresh.Execution)
	ok, err = s.finish(ctx, fresh, successSample())
	require.NoError(t, err)
	require.True(t, ok)
}
func TestInvalidationAndCleanup(t *testing.T) {
	for _, change := range []string{"pause", "interval", "connection", "inactive", "expired", "deleted"} {
		t.Run(change, func(t *testing.T) {
			s := testStore(t)
			ctx := context.Background()
			enable(t, s, 1)
			old := claimOne(t, s)
			switch change {
			case "pause":
				require.NoError(t, s.Configure(ctx, 1, Config{Enabled: false, Interval: 10}))
			case "interval":
				require.NoError(t, s.Configure(ctx, 1, Config{Enabled: true, Interval: 30}))
			case "connection":
				_, err := s.DB.Exec(`UPDATE proxies SET password='new' WHERE id=1`)
				require.NoError(t, err)
			case "inactive":
				_, err := s.DB.Exec(`UPDATE proxies SET status='inactive' WHERE id=1`)
				require.NoError(t, err)
			case "expired":
				_, err := s.DB.Exec(`UPDATE proxies SET expires_at=now()-interval '1 second' WHERE id=1`)
				require.NoError(t, err)
			case "deleted":
				_, err := s.DB.Exec(`UPDATE proxies SET deleted_at=now() WHERE id=1`)
				require.NoError(t, err)
			}
			ok, err := s.finish(ctx, old, successSample())
			require.NoError(t, err)
			require.False(t, ok)
			if change == "deleted" {
				require.NoError(t, s.cleanup(ctx))
				var n int
				require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM proxy_openai_monitor_configs`).Scan(&n))
				require.Zero(t, n)
			}
		})
	}
}
func TestSaveFailureDoesNotReprobeOldRound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	enable(t, s, 1)
	j := claimOne(t, s)
	_, err := s.DB.Exec(`CREATE FUNCTION reject_sample() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected save failure'; END $$; CREATE TRIGGER reject_sample BEFORE INSERT ON proxy_openai_monitor_samples FOR EACH ROW EXECUTE FUNCTION reject_sample()`)
	require.NoError(t, err)
	ok, err := s.finish(ctx, j, successSample())
	require.Error(t, err)
	require.False(t, ok)
	next, _, err := s.claim(ctx)
	require.NoError(t, err)
	require.Nil(t, next)
	_, err = s.DB.Exec(`DROP TRIGGER reject_sample ON proxy_openai_monitor_samples;UPDATE proxy_openai_monitor_configs SET lease_until=now()-interval '1 second',next_at=now()-interval '30 seconds'`)
	require.NoError(t, err)
	next, _, err = s.claim(ctx)
	require.NoError(t, err)
	require.Nil(t, next)
	due(t, s, 1)
	next = claimOne(t, s)
	require.NotEqual(t, j.Execution, next.Execution)
	ok, err = s.finish(ctx, next, successSample())
	require.NoError(t, err)
	require.True(t, ok)
}
func TestQueryStatisticsFingerprintAndRetention(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	enable(t, s, 1)
	j := claimOne(t, s)
	var end time.Time
	require.NoError(t, s.DB.QueryRow(`SELECT now()`).Scan(&end))
	for i, lat := range []int{100, 200, 300, -1} {
		var latency any
		success := lat >= 0
		if success {
			latency = lat
		}
		_, err := s.DB.Exec(`INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,latency_ms,http_status,error_class) VALUES(1,$1,1,$2,$3,$3,$3,$4,$5,401,'')`, j.Fingerprint, fmt.Sprintf("sample-%d", i), end.Add(-time.Duration(i+1)*time.Minute), success, latency)
		require.NoError(t, err)
	}
	r, err := s.Query(ctx, 1, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 4, r.Summary.Count)
	require.InDelta(t, 0.75, *r.Summary.Rate, 0.0001)
	require.InDelta(t, 200, *r.Summary.Average, 0.001)
	require.InDelta(t, 290, *r.Summary.P95, 0.001)
	require.Len(t, r.Trend, 60)
	require.Len(t, r.Records, 4)
	require.NotNil(t, r.Latest)
	require.NoError(t, s.Configure(ctx, 1, Config{Enabled: true, Interval: 60}))
	r, err = s.Query(ctx, 1, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 4, r.Summary.Count)
	_, err = s.DB.Exec(`UPDATE proxies SET password='changed' WHERE id=1`)
	require.NoError(t, err)
	r, err = s.Query(ctx, 1, 1, 1)
	require.NoError(t, err)
	require.Zero(t, r.Summary.Count)
	require.Nil(t, r.Summary.Average)
	require.Nil(t, r.Latest)
	require.Empty(t, r.Records)
	for _, b := range r.Trend {
		require.Zero(t, b.Count)
	}
	_, err = s.DB.Exec(`UPDATE proxy_openai_monitor_samples SET finished_at=now()-interval '8 days'`)
	require.NoError(t, err)
	require.NoError(t, s.cleanup(ctx))
	var n int
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM proxy_openai_monitor_samples`).Scan(&n))
	require.Zero(t, n)
}

type probeStub struct {
	active, peak, calls atomic.Int32
	delay               time.Duration
}

func (p *probeStub) ProbeProxy(context.Context, string) (*service.ProxyExitInfo, int64, error) {
	panic("unexpected health probe")
}
func (p *probeStub) ProbeProxyTarget(ctx context.Context, _, _ string) (int, int64, error) {
	p.calls.Add(1)
	a := p.active.Add(1)
	defer p.active.Add(-1)
	for {
		peak := p.peak.Load()
		if a <= peak || p.peak.CompareAndSwap(peak, a) {
			break
		}
	}
	select {
	case <-ctx.Done():
		return 0, 0, ctx.Err()
	case <-time.After(p.delay):
		return 401, p.delay.Milliseconds(), nil
	}
}
func TestRunnerBoundedAndShutdown(t *testing.T) {
	s := testStore(t)
	_, err := s.DB.Exec(`INSERT INTO proxies(id) SELECT generate_series(3,40)`)
	require.NoError(t, err)
	for i := 1; i <= 40; i++ {
		enable(t, s, int64(i))
	}
	p := &probeStub{delay: time.Minute}
	r := New(s, p)
	r.Start()
	deadline := time.Now().Add(5 * time.Second)
	for p.active.Load() < Workers && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	require.EqualValues(t, Workers, p.active.Load())
	start := time.Now()
	r.Stop()
	require.Less(t, time.Since(start), time.Second)
	require.Zero(t, p.active.Load())
	require.LessOrEqual(t, p.peak.Load(), int32(Workers))
	var n int
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM proxy_openai_monitor_samples`).Scan(&n))
	require.Zero(t, n)
}

func TestNoDataAllFailuresAndBoundaries(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	r, err := s.Query(ctx, 1, 24, 1)
	require.NoError(t, err)
	require.False(t, r.Config.Enabled)
	require.Equal(t, 30, r.Config.Interval)
	require.Nil(t, r.Summary.Rate)
	require.Nil(t, r.Summary.Average)
	require.True(t, r.Stale)
	enable(t, s, 1)
	j := claimOne(t, s)
	// The query's range is left-closed/right-open; future samples never enter it.
	_, err = s.DB.Exec(`INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,http_status,error_class)
 SELECT 1,$1,1,'failed-'||n,now(),now()-interval '1 minute',now(),false,503,'http_5xx' FROM generate_series(1,51) n`, j.Fingerprint)
	require.NoError(t, err)
	_, err = s.DB.Exec(`INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,latency_ms,http_status)
 VALUES(1,$1,1,'future',now(),now()+interval '1 hour',now()+interval '1 hour',true,1,200)`, j.Fingerprint)
	require.NoError(t, err)
	r, err = s.Query(ctx, 1, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 51, r.Summary.Count)
	require.EqualValues(t, 51, r.Summary.Failures)
	require.Nil(t, r.Summary.Average)
	require.Nil(t, r.Summary.P95)
	require.Equal(t, 0.0, *r.Summary.Rate)
	require.Len(t, r.Records, 50)
	require.True(t, r.Truncated)
	r, err = s.Query(ctx, 1, 1, 2)
	require.NoError(t, err)
	require.Len(t, r.Records, 1)
	require.False(t, r.Truncated)
	for _, hours := range []int{0, 2, 169} {
		_, err = s.Query(ctx, 1, hours, 1)
		require.ErrorIs(t, err, ErrInvalid)
	}
	require.ErrorIs(t, s.Configure(ctx, 1, Config{Enabled: true, Interval: 20}), ErrInvalid)
	_, err = s.DB.Exec(`UPDATE proxies SET status='inactive' WHERE id=1`)
	require.NoError(t, err)
	require.ErrorIs(t, s.Configure(ctx, 1, Config{Enabled: true, Interval: 30}), ErrInvalid)
	require.NoError(t, s.Configure(ctx, 1, Config{Enabled: false, Interval: 30}))
}

func TestQueryCapacity(t *testing.T) {
	if os.Getenv("PROXY_MONITOR_CAPACITY") != "1" {
		t.Skip("opt-in 604800 sample query check")
	}
	s := testStore(t)
	ctx := context.Background()
	_, err := s.DB.Exec(`INSERT INTO proxies(id) SELECT generate_series(3,10)`)
	require.NoError(t, err)
	_, err = s.DB.Exec(`INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,latency_ms,http_status)
 SELECT p.id,` + fingerprintSQL + `,1,p.id||'-'||n,
 now()-n*interval '9.9 seconds',now()-n*interval '9.9 seconds',now()-n*interval '9.9 seconds',true,(n%1000)+1,401
 FROM proxies p CROSS JOIN generate_series(1,60480) n`)
	require.NoError(t, err)
	_, err = s.DB.Exec(`ANALYZE proxy_openai_monitor_samples`)
	require.NoError(t, err)
	start := time.Now()
	r, err := s.Query(ctx, 1, 168, 1)
	require.NoError(t, err)
	require.EqualValues(t, 60480, r.Summary.Count)
	require.Len(t, r.Trend, 168)
	t.Logf("604800 rows / 60480 per proxy: cold 7-day report %s", time.Since(start))
	start = time.Now()
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err = s.Query(bounded, 1, 168, 1)
	require.NoError(t, err)
	t.Logf("warm 7-day report %s", time.Since(start))
	rows, err := s.DB.Query(`EXPLAIN (ANALYZE,BUFFERS) SELECT ` + metricsSQL + ` FROM proxy_openai_monitor_samples WHERE proxy_id=1 AND fingerprint=(SELECT ` + fingerprintSQL + ` FROM proxies p WHERE p.id=1) AND started_at>=now()-interval '7 days' AND started_at<now()`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		t.Log(line)
	}
	require.NoError(t, rows.Err())
}

func TestSoftDeleteRaceAndExpiredLeaseRejectWrite(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	enable(t, s, 1)
	j := claimOne(t, s)
	tx, err := s.DB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE proxies SET deleted_at=now() WHERE id=1`)
	require.NoError(t, err)
	type result struct {
		ok  bool
		err error
	}
	done := make(chan result, 1)
	go func() { ok, err := s.finish(ctx, j, successSample()); done <- result{ok, err} }()
	require.NoError(t, tx.Commit())
	out := <-done
	require.NoError(t, out.err)
	require.False(t, out.ok)
	require.NoError(t, s.cleanup(ctx))
	var n int
	require.NoError(t, s.DB.QueryRow(`SELECT count(*) FROM proxy_openai_monitor_samples`).Scan(&n))
	require.Zero(t, n)
	enable(t, s, 2)
	expired := claimOne(t, s)
	_, err = s.DB.Exec(`UPDATE proxy_openai_monitor_configs SET lease_until=clock_timestamp()-interval '1 second' WHERE proxy_id=2`)
	require.NoError(t, err)
	ok, err := s.finish(ctx, expired, successSample())
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMigrationOnEmptyDatabase(t *testing.T) {
	s := testStoreWithData(t, false)
	_, err := s.DB.Exec(`INSERT INTO proxies(id) VALUES(1)`)
	require.NoError(t, err)
	r, err := s.Query(context.Background(), 1, 1, 1)
	require.NoError(t, err)
	require.False(t, r.Config.Enabled)
	require.Zero(t, r.Summary.Count)
}

func TestBacklogAdvancementDoesNotBlockDueWork(t *testing.T) {
	s := testStore(t)
	_, err := s.DB.Exec(`INSERT INTO proxies(id) SELECT generate_series(3,21)`)
	require.NoError(t, err)
	for id := int64(1); id <= 21; id++ {
		enable(t, s, id)
	}
	// Twenty overdue configurations precede one immediately runnable job.
	_, err = s.DB.Exec(`UPDATE proxy_openai_monitor_configs SET next_at=clock_timestamp()-interval '100 seconds' WHERE proxy_id<=20`)
	require.NoError(t, err)
	p := &probeStub{delay: 10 * time.Millisecond}
	r := New(s, p)
	r.Start()
	defer r.Stop()
	require.Eventually(t, func() bool { return p.calls.Load() > 0 }, 3*time.Second, 20*time.Millisecond, "overdue schedules must not hide a fresh due job")
	// Recovery must reach the twenty old schedules, not just the fresh job.
	require.Eventually(t, func() bool {
		var n int
		err := s.DB.QueryRow(`SELECT count(DISTINCT proxy_id) FROM proxy_openai_monitor_samples WHERE proxy_id<=20`).Scan(&n)
		return err == nil && n == 20
	}, 13*time.Second, 25*time.Millisecond)
	require.LessOrEqual(t, p.peak.Load(), int32(Workers))
}

func TestConfigRemainsAvailableWithoutHistory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	enable(t, s, 1)
	_, err := s.DB.Exec(`DROP TABLE proxy_openai_monitor_samples`)
	require.NoError(t, err)
	settings, err := s.ReadConfig(ctx, 1)
	require.NoError(t, err)
	require.True(t, settings.Config.Enabled)
	require.True(t, settings.Eligible)
	_, err = s.Query(ctx, 1, 24, 1)
	require.Error(t, err)
	require.NoError(t, s.Configure(ctx, 1, Config{Enabled: false, Interval: 10}))
	settings, err = s.ReadConfig(ctx, 1)
	require.NoError(t, err)
	require.False(t, settings.Config.Enabled)
}

func TestFixedCutoffPaginationWithNewAndLateSamples(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	enable(t, s, 1)
	j := claimOne(t, s)
	_, err := s.DB.Exec(`INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,latency_ms,http_status)
 SELECT 1,$1,1,'initial-'||n,now(),now()-n*interval '10 seconds',now()-n*interval '10 seconds',true,100,401 FROM generate_series(1,60) n`, j.Fingerprint)
	require.NoError(t, err)
	first, err := s.Query(ctx, 1, 1, 1)
	require.NoError(t, err)
	require.Len(t, first.Records, 50)
	second, err := s.QueryAt(ctx, 1, 1, 2, first.To)
	require.NoError(t, err)
	require.Len(t, second.Records, 10)
	// A completed new round and a round started before the cutoff but completed
	// afterwards must not shift an existing history page.
	_, err = s.DB.Exec(`INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,latency_ms,http_status)
 VALUES(1,$1,1,'new',now(),now(),now(),true,50,200),
 (1,$1,1,'late',now(),$2::timestamptz-interval '5 seconds',now(),true,50,200)`, j.Fingerprint, first.To)
	require.NoError(t, err)
	after, err := s.QueryAt(ctx, 1, 1, 2, first.To)
	require.NoError(t, err)
	require.Equal(t, first.To, after.To)
	require.Equal(t, first.From, after.From)
	require.Equal(t, second.Records, after.Records)
	require.EqualValues(t, 60, after.Summary.Count)
	seen := map[int64]bool{}
	for _, r := range first.Records {
		seen[r.ID] = true
	}
	for _, r := range after.Records {
		require.False(t, seen[r.ID])
		seen[r.ID] = true
	}
	require.Len(t, seen, 60)
	fresh, err := s.Query(ctx, 1, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 62, fresh.Summary.Count)
	_, err = s.QueryAt(ctx, 1, 1, 1, time.Now().Add(time.Hour))
	require.ErrorIs(t, err, ErrInvalid)
	_, err = s.QueryAt(ctx, 1, 1, 1, time.Now().Add(-8*24*time.Hour))
	require.ErrorIs(t, err, ErrInvalid)
}
