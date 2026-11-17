package store

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/jameskbb/casebook/internal/domain"
)

// Backoff bounds for busy retries.
const (
	retryInitialBackoff = 10 * time.Millisecond
	retryMaxBackoff     = 250 * time.Millisecond
)

// WithWriteTx runs fn inside a BEGIN IMMEDIATE transaction on the single
// writer connection and commits when fn returns nil.
//
// When BEGIN, fn or COMMIT fails with SQLITE_BUSY or SQLITE_LOCKED the
// transaction is rolled back and the whole attempt is retried after a jittered
// backoff (10ms doubling to 250ms, times a random factor in [0.5, 1.5)). When
// the next sleep would exceed Config.RetryBudget, WithWriteTx returns
// domain.ErrBusy (the raw SQLite error is logged at Debug, never returned).
// Any other error is returned at once after rollback. Cancelling ctx stops
// the retries.
//
// fn may run more than once, so it must be safe to re-run: assign outputs
// fresh on each call and do no network or file I/O inside it.
func (db *DB) WithWriteTx(ctx context.Context, fn func(*sql.Tx) error) error {
	return db.retryBusy(ctx, "write", func() error { return db.writeOnce(ctx, fn) })
}

func (db *DB) writeOnce(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, rollback(tx))
	}
	if err := tx.Commit(); err != nil {
		return errors.Join(err, rollback(tx))
	}
	return nil
}

// retryBusy runs attempt until it succeeds, fails with a non-busy error, ctx
// ends, or the retry budget is spent (then domain.ErrBusy).
func (db *DB) retryBusy(ctx context.Context, op string, attempt func() error) error {
	start := time.Now()
	backoff := retryInitialBackoff
	for n := 1; ; n++ {
		err := attempt()
		if err == nil || !IsBusy(err) {
			return err
		}
		sleep := time.Duration(float64(backoff) * (0.5 + rand.Float64()))
		if time.Since(start)+sleep > db.cfg.RetryBudget {
			db.log.Debug("store.busy", "op", op, "attempts", n)
			return domain.ErrBusy
		}
		t := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
		backoff = min(backoff*2, retryMaxBackoff)
	}
}

// WithReadTx runs fn inside a read-only transaction on the reader pool, so
// every query in fn sees one consistent snapshot. In WAL mode readers are not
// blocked by writers.
func (db *DB) WithReadTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// rollback rolls tx back, ignoring the error for an already finished tx.
func rollback(tx *sql.Tx) error {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return err
	}
	return nil
}
