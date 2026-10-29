package proxyquality

import (
	"context"
	"log"
	"time"
)

const cleanupBatchSize = 5000

func (s *Store) cleanupBatch(ctx context.Context) (bool, error) {
	more := false
	for _, query := range []string{
		`DELETE FROM account_proxy_quality_events WHERE id IN (SELECT id FROM account_proxy_quality_events WHERE created_at<now()-interval '30 days' ORDER BY id LIMIT 5000)`,
		`DELETE FROM account_proxy_quality_observations WHERE id IN (SELECT id FROM account_proxy_quality_observations WHERE observed_at<now()-interval '1 day' ORDER BY observed_at,id LIMIT 5000)`,
	} {
		result, err := s.DB.ExecContext(ctx, query)
		if err != nil {
			return more, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return more, err
		}
		more = more || n == cleanupBatchSize
	}
	return more, nil
}

// Cleanup is independent of the evaluator and immediately revisits full batches.
func (r *Runner) runCleanup(ctx context.Context) {
	defer r.wg.Done()
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			c, cancel := context.WithTimeout(ctx, 3*time.Second)
			more, err := r.store.cleanupBatch(c)
			cancel()
			if err != nil && ctx.Err() == nil {
				log.Printf("[AccountProxyQuality] cleanup failed: %v", err)
			}
			delay := time.Minute
			if more || err != nil {
				delay = time.Second * 5
			}
			timer.Reset(delay)
		}
	}
}
