package proxyquality

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// This is the only adapter to the host's account write/outbox schema. Keep its
// CAS, route-dependent probe invalidation and outbox atomic when porting upstream.
func switchBinding(ctx context.Context, tx *sql.Tx, id, from, to int64, reason string) error {
	result, err := tx.ExecContext(ctx, `UPDATE accounts SET proxy_id=$3,
 temp_unschedulable_until=CASE WHEN $4='business_degraded' AND temp_unschedulable_reason LIKE 'upstream transport error (proxy/network): %' THEN NULL ELSE temp_unschedulable_until END,
 temp_unschedulable_reason=CASE WHEN $4='business_degraded' AND temp_unschedulable_reason LIKE 'upstream transport error (proxy/network): %' THEN NULL ELSE temp_unschedulable_reason END,
 extra=CASE WHEN type='apikey' THEN extra-'upstream_billing_probe'-'ollama_cloud_usage_snapshot'-'opencode_go_usage_snapshot' ELSE extra END,updated_at=now()
 WHERE id=$1 AND proxy_id=$2 AND deleted_at IS NULL AND platform='openai' AND status='active'
 AND parent_account_id IS NULL AND proxy_fallback_origin_id IS NULL`, id, from, to, reason)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("account binding changed")
	}
	// Shadows inherit their parent's network route in the host. Keep that
	// invariant and invalidate every affected snapshot in the same transaction.
	rows, err := tx.QueryContext(ctx, `UPDATE accounts SET proxy_id=$2,updated_at=now(),
 temp_unschedulable_until=CASE WHEN $3='business_degraded' AND temp_unschedulable_reason LIKE 'upstream transport error (proxy/network): %' THEN NULL ELSE temp_unschedulable_until END,
 temp_unschedulable_reason=CASE WHEN $3='business_degraded' AND temp_unschedulable_reason LIKE 'upstream transport error (proxy/network): %' THEN NULL ELSE temp_unschedulable_reason END
 WHERE parent_account_id=$1 AND deleted_at IS NULL AND proxy_id IS DISTINCT FROM $2 RETURNING id`, id, to, reason)
	if err != nil {
		return err
	}
	changed := []int64{id}
	for rows.Next() {
		var child int64
		if err = rows.Scan(&child); err != nil {
			rows.Close()
			return err
		}
		changed = append(changed, child)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, accountID := range changed {
		_, err = tx.ExecContext(ctx, `INSERT INTO scheduler_outbox(event_type,account_id,payload) VALUES($1,$2,'{}'::jsonb)`, service.SchedulerOutboxEventAccountChanged, accountID)
		if err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO account_proxy_quality_events(account_id,from_proxy_id,to_proxy_id,reason) VALUES($1,$2,$3,$4)`, id, from, to, reason)
	return err
}
