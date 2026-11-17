// Package config loads service settings. Precedence, highest first:
// command-line flags, environment variables (SVC_*), the config file, then
// built-in defaults.
package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Config holds the service settings.
type Config struct {
	Port     int
	LogLevel string
	DBURL    string
}

// Defaults are the built-in settings.
func Defaults() Config {
	return Config{Port: 8080, LogLevel: "info", DBURL: "postgres://localhost/svc"}
}

// Load builds the configuration from defaults, the environment, the file at
// path (skipped when empty or missing) and flag overrides.
func Load(path string, flags map[string]string) (Config, error) {
	cfg := Defaults()
	applyEnv(&cfg, os.Environ())
	if path != "" {
		fileVals, err := readFile(path)
		if err != nil && !os.IsNotExist(err) {
			return Config{}, err
		}
		apply(&cfg, fileVals)
	}
	apply(&cfg, flags)
	return cfg, nil
}

// applyEnv applies SVC_* environment variables.
func applyEnv(cfg *Config, environ []string) {
	vals := map[string]string{}
	for _, kv := range environ {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(k, "SVC_") {
			continue
		}
		vals[strings.ToLower(strings.TrimPrefix(k, "SVC_"))] = v
	}
	apply(cfg, vals)
}

// apply sets the known keys present in vals.
func apply(cfg *Config, vals map[string]string) {
	if v, ok := vals["port"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Port = n
		}
	}
	if v, ok := vals["log_level"]; ok {
		cfg.LogLevel = v
	}
	if v, ok := vals["db_url"]; ok {
		cfg.DBURL = v
	}
}

// readFile reads key = value lines; # starts a comment.
func readFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	vals := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		vals[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return vals, sc.Err()
}
