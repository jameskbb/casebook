package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg, err := Load("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8080 {
		t.Fatalf("port = %d", cfg.Port)
	}
}

func TestFlagWins(t *testing.T) {
	p := filepath.Join(t.TempDir(), "svc.conf")
	if err := os.WriteFile(p, []byte("port = 7000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p, map[string]string{"port": "6000"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 6000 {
		t.Fatalf("port = %d", cfg.Port)
	}
}

func TestEnvOverridesDefault(t *testing.T) {
	t.Setenv("SVC_PORT", "9090")
	cfg, err := Load("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9090 {
		t.Fatalf("port = %d", cfg.Port)
	}
}
