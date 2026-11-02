package app_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/jameskbb/casebook/internal/app"
)

func TestOpenImportFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inv.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := app.OpenImportFile(path)
	if err != nil {
		t.Fatalf("regular file: %v", err)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil || string(data) != "{}" {
		t.Fatalf("read = %q, %v", data, err)
	}

	_, err = app.OpenImportFile(dir)
	ve, ok := app.IsValidation(err)
	if !ok || ve.Field != "data" || ve.Message != "must be a regular file" {
		t.Fatalf("directory = %v, want validation on data", err)
	}

	if _, err := app.OpenImportFile(filepath.Join(dir, "missing.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing = %v, want not exist", err)
	}
}
