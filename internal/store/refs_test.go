package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jameskbb/casebook/internal/domain"
)

// TestAllocationRefusesPastMaxRefNum proves allocation never returns a ref
// number that ParseInvestigationRef or ParseRecordRef would reject.
func TestAllocationRefusesPastMaxRefNum(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	var invID int64
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		invID, err = insertTestInvestigation(ctx, tx, "Full")
		return err
	})

	// The last allowed numbers still allocate and parse.
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE counters SET value = ? WHERE name = 'investigation'`,
			domain.MaxRefNum-1); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE investigations SET last_evidence_num = ? WHERE id = ?`,
			domain.MaxRefNum-1, invID)
		return err
	})
	mustWrite(t, db, func(ctx context.Context, tx *sql.Tx) error {
		n, err := AllocateInvestigationNum(ctx, tx)
		if err != nil || n != domain.MaxRefNum {
			t.Fatalf("AllocateInvestigationNum = %d, %v; want %d", n, err, int64(domain.MaxRefNum))
		}
		if _, err := domain.ParseInvestigationRef(domain.FormatInvestigationRef(n)); err != nil {
			t.Fatalf("last investigation ref does not parse: %v", err)
		}
		n, err = AllocateRecordNum(ctx, tx, invID, domain.RecordEvidence)
		if err != nil || n != domain.MaxRefNum {
			t.Fatalf("AllocateRecordNum = %d, %v; want %d", n, err, int64(domain.MaxRefNum))
		}
		if _, err := domain.ParseRecordRef(domain.FormatRecordRef(domain.RecordEvidence, n), domain.RecordEvidence); err != nil {
			t.Fatalf("last evidence ref does not parse: %v", err)
		}
		return nil
	})

	// One more is refused with ErrConflict and rolls back.
	err := db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		_, err := AllocateInvestigationNum(ctx, tx)
		return err
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("AllocateInvestigationNum past the max = %v, want ErrConflict", err)
	}
	err = db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		_, err := AllocateRecordNum(ctx, tx, invID, domain.RecordEvidence)
		return err
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("AllocateRecordNum past the max = %v, want ErrConflict", err)
	}
	var counter, last int64
	if err := db.reader.QueryRowContext(ctx, `SELECT value FROM counters WHERE name = 'investigation'`).Scan(&counter); err != nil {
		t.Fatal(err)
	}
	if err := db.reader.QueryRowContext(ctx, `SELECT last_evidence_num FROM investigations WHERE id = ?`, invID).Scan(&last); err != nil {
		t.Fatal(err)
	}
	if counter != domain.MaxRefNum || last != domain.MaxRefNum {
		t.Fatalf("refused allocation moved counters: %d, %d", counter, last)
	}
}
