package store

import (
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// primaryCode returns the primary SQLite result code of err (extended codes
// masked to their low byte), or 0 when err is not an SQLite error.
func primaryCode(err error) int {
	var se *sqlite.Error
	if errors.As(err, &se) {
		return se.Code() & 0xff
	}
	return 0
}

// IsBusy reports whether err is SQLITE_BUSY or SQLITE_LOCKED (lock
// contention). It inspects SQLite result codes, never error text.
func IsBusy(err error) bool {
	c := primaryCode(err)
	return c == sqlite3.SQLITE_BUSY || c == sqlite3.SQLITE_LOCKED
}

// IsConstraint reports whether err is an SQLITE_CONSTRAINT failure (CHECK,
// NOT NULL, UNIQUE, foreign key, or a trigger RAISE(ABORT)).
func IsConstraint(err error) bool {
	return primaryCode(err) == sqlite3.SQLITE_CONSTRAINT
}

// IsSQLite reports whether err came from SQLite.
func IsSQLite(err error) bool {
	return primaryCode(err) != 0
}
