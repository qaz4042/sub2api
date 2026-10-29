package repository

import (
	"context"
	"database/sql"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Serialize shadow insertion with parent proxy updates and read the binding
// under that lock, so a stale service-layer parent snapshot cannot leak out.
func (r *accountRepository) createShadowWithInheritedProxy(ctx context.Context, account *service.Account) error {
	client := clientFromContext(ctx, r.client)
	var tx *dbent.Tx
	if dbent.TxFromContext(ctx) == nil {
		var err error
		tx, err = r.client.Tx(ctx)
		if err != nil && err != dbent.ErrTxStarted {
			return err
		}
		if tx != nil {
			defer tx.Rollback()
			ctx = dbent.NewTxContext(ctx, tx)
			client = tx.Client()
		}
	}
	rows, err := client.QueryContext(ctx, `SELECT proxy_id FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, *account.ParentAccountID)
	if err != nil {
		return err
	}
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		return service.ErrAccountNotFound
	}
	var proxy sql.NullInt64
	err = rows.Scan(&proxy)
	rows.Close()
	if err != nil {
		return err
	}
	if proxy.Valid {
		id := proxy.Int64
		account.ProxyID = &id
	} else {
		account.ProxyID = nil
	}
	if err = createAccountRecord(ctx, client, account); err != nil {
		return err
	}
	if err = enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &account.ID, nil, buildSchedulerGroupPayload(account.GroupIDs)); err != nil {
		return err
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}
