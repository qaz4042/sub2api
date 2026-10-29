package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestShadowCreateUsesLockedParentProxy(t *testing.T) {
	dsn := os.Getenv("PROXY_QUALITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set PROXY_QUALITY_TEST_DSN to a disposable PostgreSQL database")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("shadow_create_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close(); admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); admin.Close() })
	_, err = db.Exec(`CREATE TABLE accounts(id bigserial PRIMARY KEY,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),deleted_at timestamptz,
 name text NOT NULL,notes text,platform text NOT NULL,type text NOT NULL,credentials jsonb NOT NULL DEFAULT '{}',extra jsonb NOT NULL DEFAULT '{}',concurrency int NOT NULL DEFAULT 1,
 priority int NOT NULL DEFAULT 1,rate_multiplier numeric NOT NULL DEFAULT 1,status text NOT NULL DEFAULT 'active',error_message text NOT NULL DEFAULT '',schedulable boolean NOT NULL DEFAULT true,
 auto_pause_on_expired boolean NOT NULL DEFAULT true,proxy_id bigint,parent_account_id bigint,quota_dimension text NOT NULL DEFAULT 'global',temp_unschedulable_until timestamptz,temp_unschedulable_reason text);
 CREATE TABLE scheduler_outbox(id bigserial PRIMARY KEY,event_type text NOT NULL,account_id bigint,group_id bigint,payload jsonb,dedup_key text);
 CREATE UNIQUE INDEX scheduler_outbox_dedup ON scheduler_outbox(dedup_key) WHERE dedup_key IS NOT NULL;
 INSERT INTO accounts(id,name,platform,type,proxy_id) VALUES(1,'parent','openai','oauth',1);
 SELECT setval(pg_get_serial_sequence('accounts','id'),1)`)
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := newAccountRepositoryWithSQL(client, db, nil)
	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE accounts SET proxy_id=2 WHERE id=1`)
	require.NoError(t, err)
	parentID, old := int64(1), int64(1)
	shadow := &service.Account{Name: "shadow", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, ParentAccountID: &parentID, ProxyID: &old, QuotaDimension: service.QuotaDimensionSpark}
	done := make(chan error, 1)
	go func() { done <- repo.Create(context.Background(), shadow) }()
	require.Eventually(t, func() bool {
		var count int
		err := db.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT proxy_id FROM accounts WHERE id=%'`).Scan(&count)
		return err == nil && count > 0
	}, 3*time.Second, 20*time.Millisecond)
	require.NoError(t, tx.Commit())
	require.NoError(t, <-done)
	require.NotNil(t, shadow.ProxyID)
	require.EqualValues(t, 2, *shadow.ProxyID)
	var actual int64
	require.NoError(t, db.QueryRow(`SELECT proxy_id FROM accounts WHERE id=$1`, shadow.ID).Scan(&actual))
	require.EqualValues(t, 2, actual)
	until := time.Now().Add(time.Minute)
	updated, err := repo.SetTempUnschedulableForProxy(context.Background(), parentID, &old, until, "old route")
	require.NoError(t, err)
	require.False(t, updated)
	current := int64(2)
	updated, err = repo.SetTempUnschedulableForProxy(context.Background(), parentID, &current, until, "new route")
	require.NoError(t, err)
	require.True(t, updated)
	outer, err := client.Tx(context.Background())
	require.NoError(t, err)
	stale := int64(1)
	second := &service.Account{Name: "transactional-shadow", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, ParentAccountID: &parentID, ProxyID: &stale, QuotaDimension: service.QuotaDimensionSpark}
	require.NoError(t, repo.Create(dbent.NewTxContext(context.Background(), outer), second))
	require.EqualValues(t, 2, *second.ProxyID)
	require.NoError(t, outer.Rollback())
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE id=$1`, second.ID).Scan(&count))
	require.Zero(t, count)
}
