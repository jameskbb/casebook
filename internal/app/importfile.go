package app

import (
	"os"

	"github.com/jameskbb/casebook/internal/domain"
)

// OpenImportFile opens path for an import. Anything but a regular file is a
// ValidationError on data: a FIFO or device could block the read forever and
// a directory has nothing to read. The path is checked before it is opened
// and the opened file is checked again, so a file swapped in between is
// refused too. On unix the open itself does not block (openNoBlock), so a
// FIFO swapped in between the two checks is refused rather than waiting for
// a writer. Callers bound the read (ImportInvestigationFrom does). A missing
// or unreadable path returns the os error as is.
func OpenImportFile(path string) (*os.File, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, notRegular()
	}
	if beforeImportOpen != nil {
		beforeImportOpen()
	}
	f, err := openNoBlock(path)
	if err != nil {
		return nil, err
	}
	if st, err = f.Stat(); err != nil || !st.Mode().IsRegular() {
		f.Close()
		if err != nil {
			return nil, err
		}
		return nil, notRegular()
	}
	return f, nil
}

// beforeImportOpen, when set by a test, runs between OpenImportFile's path
// check and its open, so the test can swap the file.
var beforeImportOpen func()

func notRegular() error {
	return domain.NewValidationError("data", "must be a regular file")
}
