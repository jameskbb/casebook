package app

// SetFoldDirCase forces case-insensitive (true) or exact (false) directory
// binding matches for a test and returns a func restoring the default.
func SetFoldDirCase(v bool) (restore func()) {
	old := foldDirCase
	foldDirCase = v
	return func() { foldDirCase = old }
}
