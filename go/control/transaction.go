package control

import (
	"context"
	"database/sql"
)

// WithTransaction pins the connection until failed COMMIT cleanup finishes.
// SQLite retains a transaction after some COMMIT failures (e.g. deferred FKs),
// but database/sql marks sql.Tx done. A subsequent tx.Rollback is then a no-op.
// Explicit ROLLBACK on the pinned connection prevents state leaking to its next
// borrower. The original operation error is preserved.
func WithTransaction(ctx context.Context, db *sql.DB, action func(*sql.Tx) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = action(tx); err != nil {
		return err
	}
	return commitPinned(ctx, conn, tx)
}
func commitPinned(ctx context.Context, conn *sql.Conn, tx *sql.Tx) error {
	if err := tx.Commit(); err != nil {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		return err
	}
	return nil
}
