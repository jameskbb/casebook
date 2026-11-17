package integrations

import (
	"os"
	"testing"

	"github.com/jameskbb/casebook/internal/testenv"
)

// TestMain resolves the temp directory first, so expected paths built from
// t.TempDir() match the resolved paths setup reports on macOS and Windows.
func TestMain(m *testing.M) {
	testenv.CanonicalTempDir()
	os.Exit(m.Run())
}
