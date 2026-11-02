package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jameskbb/casebook/internal/domain"
)

// Backup support (D-22, D-24): VACUUM INTO snapshots and strict read-only
// validation of snapshot files, which may be corrupt, from a newer build or
// deliberately crafted.

// ErrDestinationExists means a backup destination file already exists.
// Backups never overwrite a file.
var ErrDestinationExists = errors.New("store: backup destination already exists")

// ErrInvalidSnapshot means a file is not a genuine, untampered Casebook
// snapshot. Errors wrapping it are *SnapshotError values carrying a short
// reason that names tables or columns only, never raw SQLite text or data.
var ErrInvalidSnapshot = errors.New("store: invalid Casebook snapshot")

// SnapshotError is a validation failure with a short reason.
type SnapshotError struct {
	Reason string
}

func (e *SnapshotError) Error() string { return ErrInvalidSnapshot.Error() + ": " + e.Reason }

// Unwrap makes errors.Is(err, ErrInvalidSnapshot) true.
func (e *SnapshotError) Unwrap() error { return ErrInvalidSnapshot }

func invalidSnapshot(format string, args ...any) error {
	return &SnapshotError{Reason: fmt.Sprintf(format, args...)}
}

// SnapshotReason returns the short reason of a *SnapshotError in err's chain,
// or "" when there is none.
func SnapshotReason(err error) string {
	var se *SnapshotError
	if errors.As(err, &se) {
		return se.Reason
	}
	return ""
}

// VacuumInto writes a consistent snapshot of the database to dest with
// VACUUM INTO. dest is created first with O_CREATE|O_EXCL and mode 0600, so
// an existing file is never overwritten (ErrDestinationExists) and the
// snapshot is never readable by others, even briefly. VACUUM INTO runs on the
// reader pool outside any transaction, so it sees one snapshot while other
// connections keep writing. On failure dest is removed. The result is a
// single file in rollback (delete) journal mode.
func (db *DB) VacuumInto(ctx context.Context, dest string) error {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return ErrDestinationExists
	}
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(dest)
		return err
	}
	if _, err := db.reader.ExecContext(ctx, `VACUUM INTO ?`, dest); err != nil {
		os.Remove(dest)
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dest, 0o600); err != nil {
			os.Remove(dest)
			return err
		}
	}
	return nil
}

// OpenReadOnly opens a snapshot file for validation: read-only and immutable
// (SQLite never writes, locks or creates -wal/-shm/-journal files next to
// it), query_only on, trusted_schema off (functions in a crafted schema never
// run), and a single connection. The caller must Close it.
func OpenReadOnly(path string) (*sql.DB, error) {
	dsn := fileURI(path) + "?mode=ro&immutable=1&_pragma=query_only(1)&_pragma=trusted_schema(0)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// ValidateSnapshot checks an open snapshot against the embedded migrations,
// in this order:
//
//   - schema_migrations exists with exactly the Casebook definition and
//     1 <= max(version) <= latest;
//   - the schema fingerprint equals a reference built from the embedded
//     migrations at that version (so extra triggers, views or indexes are
//     refused);
//   - PRAGMA integrity_check returns exactly "ok" (this also verifies CHECK
//     and NOT NULL constraints);
//   - PRAGMA foreign_key_check returns no rows;
//   - no text column holds a NUL byte or exceeds its Casebook limit;
//   - the cross-row invariants the engine relies on hold: ref numbers and
//     counters within domain.MaxRefNum, links and superseded_by within one
//     investigation, and status fields consistent with their companions
//     (snapshotInvariants).
//
// The schema is compared first because reading sqlite_schema evaluates
// nothing, while integrity_check evaluates every CHECK constraint and every
// expression or partial index for every row: only a schema identical to the
// embedded migrations is ever checked row by row, so a crafted file cannot
// make validation run its expressions.
//
// It returns the schema version and row counts. Every failure wraps
// ErrInvalidSnapshot with a short reason.
func ValidateSnapshot(ctx context.Context, db *sql.DB, latest int) (int, domain.Counts, error) {
	return validateSnapshot(ctx, db, nil, latest)
}

// reference returns the reference fingerprint at version for migrations fsys
// (nil means the embedded migrations, whose fingerprints are cached).
func reference(ctx context.Context, fsys fs.FS, version int) ([]SchemaObject, error) {
	if fsys == nil {
		return embeddedReference(ctx, version)
	}
	return ReferenceFingerprint(ctx, fsys, version)
}

// findObject returns the object with type typ and name in objs.
func findObject(objs []SchemaObject, typ, name string) (SchemaObject, bool) {
	for _, o := range objs {
		if o.Type == typ && o.Name == name {
			return o, true
		}
	}
	return SchemaObject{}, false
}

// validateSnapshot is ValidateSnapshot against migrations fsys (nil means the
// embedded migrations, whose reference fingerprints are cached).
func validateSnapshot(ctx context.Context, q Querier, fsys fs.FS, latest int) (int, domain.Counts, error) {
	var none domain.Counts

	got, err := SchemaFingerprint(ctx, q)
	if err != nil {
		return 0, none, readFailure(ctx, err)
	}
	gotMigrations, ok := findObject(got, "table", "schema_migrations")
	if !ok {
		return 0, none, invalidSnapshot("no schema_migrations table")
	}
	// Check the schema_migrations definition before reading the version, so
	// a crafted definition (for example a generated column) is never
	// evaluated.
	latestRef, err := reference(ctx, fsys, latest)
	if err != nil {
		return 0, none, err
	}
	if wantMigrations, ok := findObject(latestRef, "table", "schema_migrations"); !ok || wantMigrations != gotMigrations {
		return 0, none, invalidSnapshot("schema differs from the Casebook schema (table schema_migrations differs)")
	}
	var v sql.NullInt64
	if err := q.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, none, invalidSnapshot("unreadable schema_migrations")
	}
	version := int(v.Int64)
	if !v.Valid || version < 1 || version > latest {
		return 0, none, invalidSnapshot("schema version %d is not supported (this build reads 1 to %d)", v.Int64, latest)
	}
	want, err := reference(ctx, fsys, version)
	if err != nil {
		return 0, none, err
	}
	if diff := fingerprintDiff(got, want); diff != "" {
		return 0, none, invalidSnapshot("schema differs from Casebook schema version %d (%s)", version, diff)
	}

	ok, err = integrityOK(ctx, q)
	if err != nil {
		return 0, none, readFailure(ctx, err)
	}
	if !ok {
		return 0, none, invalidSnapshot("integrity check failed")
	}
	if table, err := firstForeignKeyViolation(ctx, q); err != nil {
		return 0, none, readFailure(ctx, err)
	} else if table != "" {
		return 0, none, invalidSnapshot("foreign key violation in %s", table)
	}

	limits := slices.Clone(snapshotLimits)
	if version >= 8 {
		limits = append(limits, snapshotLimit{"investigations", "conclusion_review_basis", domain.MaxConclusionRationaleBytes})
	}
	for _, l := range limits {
		var bad bool
		// Table and column names come from the static snapshotLimits list.
		err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM `+l.table+` WHERE length(CAST(`+l.column+
			` AS BLOB)) > ? OR instr(CAST(`+l.column+` AS BLOB), X'00') > 0)`, l.max).Scan(&bad)
		if err != nil {
			return 0, none, readFailure(ctx, err)
		}
		if bad {
			return 0, none, invalidSnapshot("%s.%s holds data outside Casebook limits", l.table, l.column)
		}
	}

	if err := checkInvariants(ctx, q, version); err != nil {
		return 0, none, err
	}

	counts, err := CountAll(ctx, q)
	if err != nil {
		return 0, none, readFailure(ctx, err)
	}
	return version, counts, nil
}

// snapshotInvariant is one cross-row rule the engine relies on but the
// schema's constraints and triggers do not enforce for rows already in a
// file (the link and supersede guard triggers only fire on writes). query is
// static SQL returning 1 when the rule is broken.
type snapshotInvariant struct {
	reason string
	query  string
}

// snapshotInvariants lists the rules checkInvariants enforces. Low counters
// are not listed: ReplaceAll repairs them (repairCounters), so a backup whose
// counters lag behind its refs restores fine and never reuses a ref.
var snapshotInvariants = []snapshotInvariant{
	{"investigations.ref_num is out of range",
		`SELECT EXISTS (SELECT 1 FROM investigations WHERE ref_num > ` + maxRefNumSQL + `)`},
	{"investigations ref counters are out of range",
		`SELECT EXISTS (SELECT 1 FROM investigations WHERE
  last_evidence_num NOT BETWEEN 0 AND ` + maxRefNumSQL + ` OR last_claim_num NOT BETWEEN 0 AND ` + maxRefNumSQL + `
  OR last_hypothesis_num NOT BETWEEN 0 AND ` + maxRefNumSQL + ` OR last_question_num NOT BETWEEN 0 AND ` + maxRefNumSQL + `)`},
	{"counters holds a value out of range",
		`SELECT EXISTS (SELECT 1 FROM counters WHERE value NOT BETWEEN 0 AND ` + maxRefNumSQL + `)`},
	{"evidence.ref_num is out of range",
		`SELECT EXISTS (SELECT 1 FROM evidence WHERE ref_num > ` + maxRefNumSQL + `)`},
	{"claims.ref_num is out of range",
		`SELECT EXISTS (SELECT 1 FROM claims WHERE ref_num > ` + maxRefNumSQL + `)`},
	{"hypotheses.ref_num is out of range",
		`SELECT EXISTS (SELECT 1 FROM hypotheses WHERE ref_num > ` + maxRefNumSQL + `)`},
	{"questions.ref_num is out of range",
		`SELECT EXISTS (SELECT 1 FROM questions WHERE ref_num > ` + maxRefNumSQL + `)`},
	{"claim_evidence links records of different investigations",
		`SELECT EXISTS (SELECT 1 FROM claim_evidence l JOIN claims c ON c.id = l.claim_id
  JOIN evidence e ON e.id = l.evidence_id WHERE c.investigation_id != e.investigation_id)`},
	{"hypothesis_evidence links records of different investigations",
		`SELECT EXISTS (SELECT 1 FROM hypothesis_evidence l JOIN hypotheses h ON h.id = l.hypothesis_id
  JOIN evidence e ON e.id = l.evidence_id WHERE h.investigation_id != e.investigation_id)`},
	{"claims.superseded_by_id names a claim of another investigation or itself",
		`SELECT EXISTS (SELECT 1 FROM claims a JOIN claims b ON b.id = a.superseded_by_id
  WHERE a.investigation_id != b.investigation_id OR a.id = b.id)`},
	{"hypotheses.superseded_by_id names a hypothesis of another investigation or itself",
		`SELECT EXISTS (SELECT 1 FROM hypotheses a JOIN hypotheses b ON b.id = a.superseded_by_id
  WHERE a.investigation_id != b.investigation_id OR a.id = b.id)`},
	{"claims.superseded_by_id is not set exactly when status is superseded",
		`SELECT EXISTS (SELECT 1 FROM claims WHERE (status = 'superseded') != (superseded_by_id IS NOT NULL))`},
	{"hypotheses.superseded_by_id is not set exactly when status is superseded",
		`SELECT EXISTS (SELECT 1 FROM hypotheses WHERE (status = 'superseded') != (superseded_by_id IS NOT NULL))`},
	{"investigations.archived_at is not set exactly when status is archived",
		`SELECT EXISTS (SELECT 1 FROM investigations WHERE (status = 'archived') != (archived_at IS NOT NULL))`},
	{"questions hold a resolution or defer note their status does not allow",
		`SELECT EXISTS (SELECT 1 FROM questions WHERE
  (status = 'resolved') != (resolved_at IS NOT NULL)
  OR (status = 'resolved' AND resolution = '')
  OR (status != 'resolved' AND resolution != '')
  OR (status != 'deferred' AND defer_note != ''))`},
}

// snapshotInvariantsSince lists rules that hold only from a schema version
// on: files written before it may break them and the migration to it
// repairs them, so they are checked only for files already at that version.
var snapshotInvariantsSince = map[int][]snapshotInvariant{
	// ADR-0013: migration 0002 repairs older files.
	2: {
		{"claims.status and claims.verification disagree about superseded or disputed",
			`SELECT EXISTS (SELECT 1 FROM claims WHERE (status = 'superseded') != (verification = 'superseded')
  OR (status = 'disputed') != (verification = 'disputed'))`},
		{"hypotheses.status and hypotheses.verification disagree about superseded",
			`SELECT EXISTS (SELECT 1 FROM hypotheses WHERE (status = 'superseded') != (verification = 'superseded'))`},
	},
	8: {
		{"conclusion claim selections exceed their limits or repeat a claim",
			`SELECT EXISTS (SELECT 1 FROM investigations i WHERE
  json_array_length(i.conclusion_supporting_claims) > 100
  OR json_array_length(i.conclusion_contradicting_claims) > 100
  OR json_array_length(i.conclusion_supporting_claims) != (SELECT count(DISTINCT value) FROM json_each(i.conclusion_supporting_claims))
  OR json_array_length(i.conclusion_contradicting_claims) != (SELECT count(DISTINCT value) FROM json_each(i.conclusion_contradicting_claims)))`},
		{"conclusion claim selections name a missing claim or a claim of another case",
			`SELECT EXISTS (SELECT 1 FROM investigations i
  JOIN json_each(i.conclusion_supporting_claims) r
  LEFT JOIN claims c ON c.investigation_id = i.id AND r.value = printf('C-%03d', c.ref_num)
  WHERE r.type != 'text' OR c.id IS NULL)
  OR EXISTS (SELECT 1 FROM investigations i
  JOIN json_each(i.conclusion_contradicting_claims) r
  LEFT JOIN claims c ON c.investigation_id = i.id AND r.value = printf('C-%03d', c.ref_num)
  WHERE r.type != 'text' OR c.id IS NULL)`},
		{"a conclusion claim both supports and contradicts the answer",
			`SELECT EXISTS (SELECT 1 FROM investigations i
  JOIN json_each(i.conclusion_supporting_claims) s
  JOIN json_each(i.conclusion_contradicting_claims) c ON s.value = c.value)`},
		{"conclusion review metadata is incomplete",
			`SELECT EXISTS (SELECT 1 FROM investigations WHERE
  (conclusion_reviewed_at = '' AND (conclusion_review_basis != '' OR conclusion_review_snapshot != ''))
  OR (conclusion_reviewed_at != '' AND (conclusion_review_basis = '' OR length(conclusion_review_snapshot) != 64
    OR conclusion_review_snapshot GLOB '*[^0-9a-f]*'))
  OR (conclusion = '' AND (conclusion_verification != 'ai_generated'
    OR json_array_length(conclusion_supporting_claims) != 0
    OR json_array_length(conclusion_contradicting_claims) != 0
    OR conclusion_reviewed_at != '')))`},
	},
}

// maxRefNumSQL is domain.MaxRefNum as an SQL literal.
var maxRefNumSQL = strconv.FormatInt(domain.MaxRefNum, 10)

// checkInvariants refuses a snapshot that breaks a cross-row rule (see
// snapshotInvariants), so a crafted file fails validation before a restore
// writes its safety copy or opens its write transaction.
func checkInvariants(ctx context.Context, q Querier, version int) error {
	rules := slices.Clone(snapshotInvariants)
	for v := 1; v <= version; v++ {
		rules = append(rules, snapshotInvariantsSince[v]...)
	}
	for _, inv := range rules {
		var bad bool
		if err := q.QueryRowContext(ctx, inv.query).Scan(&bad); err != nil {
			return readFailure(ctx, err)
		}
		if bad {
			return invalidSnapshot("%s", inv.reason)
		}
	}
	return nil
}

// readFailure maps a query error during validation: context errors pass
// through; anything else (not a database, corrupt pages, unreadable file)
// becomes a short reason with no SQLite text.
func readFailure(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return invalidSnapshot("not a readable SQLite database")
}

// integrityOK reports whether PRAGMA integrity_check returns exactly one row,
// "ok".
func integrityOK(ctx context.Context, q Querier) (bool, error) {
	rows, err := q.QueryContext(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return false, err
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return len(lines) == 1 && lines[0] == "ok", nil
}

// firstForeignKeyViolation returns the table of the first foreign key
// violation, or "".
func firstForeignKeyViolation(ctx context.Context, q Querier) (string, error) {
	rows, err := q.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if !rows.Next() {
		return "", rows.Err()
	}
	var (
		table, parent string
		rowid         sql.NullInt64
		fkid          int64
	)
	if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
		return "", err
	}
	if !safeIdent.MatchString(table) {
		table = "a table"
	}
	return table, nil
}

// safeIdent matches identifiers safe to echo in a reason.
var safeIdent = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,63}$`)

// snapshotLimit is a byte limit for one text column.
type snapshotLimit struct {
	table, column string
	max           int
}

// snapshotLimits lists every free-text column with its Casebook limit
// (domain Max*Bytes). Activity action and target_ref use the import limits.
var snapshotLimits = []snapshotLimit{
	{"investigations", "title", domain.MaxTitleBytes},
	{"investigations", "problem_statement", domain.MaxProblemStatementBytes},
	{"investigations", "conclusion", domain.MaxConclusionBytes},
	{"investigations", "conclusion_rationale", domain.MaxConclusionRationaleBytes},
	{"investigations", "created_by_client", domain.MaxClientBytes},
	{"evidence", "title", domain.MaxTitleBytes},
	{"evidence", "summary", domain.MaxEvidenceSummaryBytes},
	{"evidence", "raw_detail", domain.MaxRawDetailBytes},
	{"evidence", "source_ref", domain.MaxSourceRefBytes},
	{"evidence", "source_label", domain.MaxSourceLabelBytes},
	{"evidence", "metadata", domain.MaxMetadataBytes},
	{"evidence", "created_by_client", domain.MaxClientBytes},
	{"claims", "statement", domain.MaxStatementBytes},
	{"claims", "confidence_rationale", domain.MaxRationaleBytes},
	{"claims", "created_by_client", domain.MaxClientBytes},
	{"hypotheses", "statement", domain.MaxStatementBytes},
	{"hypotheses", "rationale", domain.MaxRationaleBytes},
	{"hypotheses", "created_by_client", domain.MaxClientBytes},
	{"questions", "question", domain.MaxQuestionBytes},
	{"questions", "resolution", domain.MaxResolutionBytes},
	{"questions", "defer_note", domain.MaxDeferNoteBytes},
	{"questions", "created_by_client", domain.MaxClientBytes},
	{"claim_evidence", "created_by_client", domain.MaxClientBytes},
	{"hypothesis_evidence", "created_by_client", domain.MaxClientBytes},
	{"activity", "actor_client", domain.MaxClientBytes},
	{"activity", "action", 64},
	{"activity", "target_ref", 64},
	{"activity", "summary", domain.MaxActivitySummaryBytes},
	{"activity", "note", domain.MaxCheckpointNoteBytes},
}

// SchemaObject is one sqlite_schema row with whitespace in SQL collapsed.
type SchemaObject struct {
	Type    string
	Name    string
	TblName string
	SQL     string
}

// SchemaFingerprint lists every schema object except SQLite's own internal
// ones, ordered by type and name. Internal objects are matched exactly, never
// with LIKE (whose "_" is a wildcard): automatic indexes (type index, name
// starting with the literal "sqlite_autoindex_", no SQL), and the tables
// sqlite_sequence, sqlite_stat1 and sqlite_stat4. Every other object is
// listed, including a crafted trigger or view whose name starts with
// "sqlite", so the fingerprint comparison refuses it.
func SchemaFingerprint(ctx context.Context, q Querier) ([]SchemaObject, error) {
	rows, err := q.QueryContext(ctx, `SELECT type, name, tbl_name, COALESCE(sql, '') FROM sqlite_schema
WHERE NOT (
  (type = 'index' AND substr(name, 1, 17) = 'sqlite_autoindex_' AND sql IS NULL)
  OR (type = 'table' AND name IN ('sqlite_sequence', 'sqlite_stat1', 'sqlite_stat4'))
) ORDER BY type, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SchemaObject{}
	for rows.Next() {
		var o SchemaObject
		if err := rows.Scan(&o.Type, &o.Name, &o.TblName, &o.SQL); err != nil {
			return nil, err
		}
		o.SQL = strings.Join(strings.Fields(o.SQL), " ")
		out = append(out, o)
	}
	return out, rows.Err()
}

// fingerprintDiff describes the first difference between got and want using
// object types and names only, or returns "" when they are equal.
func fingerprintDiff(got, want []SchemaObject) string {
	key := func(o SchemaObject) string { return o.Type + " " + o.Name }
	wantBy := make(map[string]SchemaObject, len(want))
	for _, o := range want {
		wantBy[key(o)] = o
	}
	gotBy := make(map[string]bool, len(got))
	for _, o := range got {
		gotBy[key(o)] = true
		w, ok := wantBy[key(o)]
		switch {
		case !ok:
			return "unexpected " + describeObject(o)
		case w != o:
			return describeObject(o) + " differs"
		}
	}
	for _, o := range want {
		if !gotBy[key(o)] {
			return "missing " + describeObject(o)
		}
	}
	return ""
}

// describeObject names a schema object for a reason, echoing its name only
// when it is a plain identifier.
func describeObject(o SchemaObject) string {
	typ := o.Type
	if !safeIdent.MatchString(typ) {
		typ = "object"
	}
	if safeIdent.MatchString(o.Name) {
		return typ + " " + o.Name
	}
	return typ
}

// ReferenceFingerprint migrates a throwaway in-memory database with the
// migrations in fsys up to exactly version and returns its fingerprint.
func ReferenceFingerprint(ctx context.Context, fsys fs.FS, version int) ([]SchemaObject, error) {
	w, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	defer w.Close()
	// One connection that never closes: each connection is its own memory
	// database.
	w.SetMaxOpenConns(1)
	w.SetMaxIdleConns(1)
	w.SetConnMaxLifetime(0)
	w.SetConnMaxIdleTime(0)
	fixed := func() time.Time { return time.Unix(0, 0) }
	got, err := Migrate(ctx, w, limitFS{fsys: fsys, max: version}, fixed)
	if err != nil {
		return nil, fmt.Errorf("build reference schema %d: %w", version, err)
	}
	if got != version {
		return nil, fmt.Errorf("build reference schema %d: reached version %d", version, got)
	}
	return SchemaFingerprint(ctx, w)
}

// Cached reference fingerprints of the embedded migrations, by version.
var (
	referenceMu    sync.Mutex
	referenceCache = map[int][]SchemaObject{}
)

func embeddedReference(ctx context.Context, version int) ([]SchemaObject, error) {
	referenceMu.Lock()
	defer referenceMu.Unlock()
	if fp, ok := referenceCache[version]; ok {
		return fp, nil
	}
	fp, err := ReferenceFingerprint(ctx, migrationsFS(), version)
	if err != nil {
		return nil, err
	}
	referenceCache[version] = fp
	return fp, nil
}

// migrationsFS returns the embedded migrations with the files at the root.
func migrationsFS() fs.FS {
	sub, err := fs.Sub(Migrations, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}

// limitFS hides migration files numbered above max.
type limitFS struct {
	fsys fs.FS
	max  int
}

func (l limitFS) Open(name string) (fs.File, error) { return l.fsys.Open(name) }

func (l limitFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(l.fsys, name)
	if err != nil {
		return nil, err
	}
	out := entries[:0:0]
	for _, e := range entries {
		if m := migrationName.FindStringSubmatch(e.Name()); m != nil {
			if v, _ := strconv.Atoi(m[1]); v > l.max {
				continue
			}
		}
		out = append(out, e)
	}
	return out, nil
}
