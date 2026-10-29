package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// SetTempUnschedulableForProxy rejects an old in-flight transport error after
// the account has already moved to a different proxy.
func (r *accountRepository) SetTempUnschedulableForProxy(ctx context.Context, id int64, proxyID *int64, until time.Time, reason string) (bool, error) {
	if scoped := r.accountTransactionScope(ctx); scoped != r {
		return scoped.SetTempUnschedulableForProxy(ctx, id, proxyID, until, reason)
	}
	result, err := r.sql.ExecContext(ctx, `UPDATE accounts SET temp_unschedulable_until=$1,temp_unschedulable_reason=$2,updated_at=now()
 WHERE id=$3 AND deleted_at IS NULL AND proxy_id IS NOT DISTINCT FROM $4
 AND (temp_unschedulable_until IS NULL OR temp_unschedulable_until<$1)`, until, reason, id, proxyID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	if err = enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventAccountChanged, &id, nil, nil); err != nil {
		logger.LegacyPrintf("repository.account", "[SchedulerOutbox] enqueue transport cooldown failed: account=%d err=%v", id, err)
	}
	r.syncSchedulerAccountSnapshot(ctx, id)
	return true, nil
}
