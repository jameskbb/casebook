package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/update"
)

// runUpdateCLI runs casebook with CheckUpdate replaced by check and
// returns the result and every Options the command passed.
func runUpdateCLI(t *testing.T, check func(context.Context, update.Options) (update.Result, error), args ...string) (cliResult, []update.Options) {
	t.Helper()
	var out, errb bytes.Buffer
	env := NewEnv(strings.NewReader(""), &out, &errb)
	env.Getenv = func(k string) string {
		if strings.HasPrefix(k, "CASEBOOK_") {
			return ""
		}
		return os.Getenv(k)
	}
	env.StdinIsTerminal = func() bool { return false }
	var calls []update.Options
	env.CheckUpdate = func(ctx context.Context, o update.Options) (update.Result, error) {
		calls = append(calls, o)
		if ctx == nil {
			t.Fatal("CheckUpdate got a nil context")
		}
		return check(ctx, o)
	}
	code := run(context.Background(), env, args)
	return cliResult{Stdout: out.String(), Stderr: errb.String(), Code: code}, calls
}

func fixedResult(state update.State, asset *update.Asset) update.Result {
	return update.Result{
		Current:     "v0.4.0",
		Latest:      "v0.5.0",
		State:       state,
		ReleaseURL:  "https://github.com/jameskbb/casebook/releases/tag/v0.5.0",
		PublishedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		Asset:       asset,
		CheckedAt:   time.Date(2026, 10, 14, 9, 30, 0, 0, time.UTC),
	}
}

var testAsset = &update.Asset{
	Name:      "casebook-cli-v0.5.0-linux-amd64.tar.gz",
	URL:       "https://github.com/jameskbb/casebook/releases/download/v0.5.0/casebook-cli-v0.5.0-linux-amd64.tar.gz",
	SizeBytes: 12345,
}

func TestUpdateHumanOutput(t *testing.T) {
	tests := []struct {
		name string
		res  update.Result
		want string
	}{
		{
			name: "up to date",
			res:  update.Result{Current: "v0.5.0", Latest: "v0.5.0", State: update.StateUpToDate, ReleaseURL: "https://example.invalid/r"},
			want: "Casebook v0.5.0 is the latest release.\n",
		},
		{
			name: "update available",
			res:  fixedResult(update.StateUpdateAvailable, testAsset),
			want: "A newer Casebook is available: v0.5.0 (you have v0.4.0).\n" +
				"Release page: https://github.com/jameskbb/casebook/releases/tag/v0.5.0\n" +
				"Download:     " + testAsset.URL + "\n" +
				"Updating Casebook: https://github.com/jameskbb/casebook/blob/main/docs/updating.md\n",
		},
		{
			name: "update available without asset",
			res:  fixedResult(update.StateUpdateAvailable, nil),
			want: "A newer Casebook is available: v0.5.0 (you have v0.4.0).\n" +
				"Release page: https://github.com/jameskbb/casebook/releases/tag/v0.5.0\n" +
				"Updating Casebook: https://github.com/jameskbb/casebook/blob/main/docs/updating.md\n",
		},
		{
			name: "ahead",
			res:  update.Result{Current: "v0.6.0-rc.1", Latest: "v0.5.0", State: update.StateAhead},
			want: "You have Casebook v0.6.0-rc.1, which is newer than the latest release, v0.5.0.\n",
		},
		{
			name: "unknown",
			res:  update.Result{Current: "dev", Latest: "v0.5.0", State: update.StateUnknown, ReleaseURL: "https://github.com/jameskbb/casebook/releases/tag/v0.5.0"},
			want: "This build (dev) is not a release version. The latest release is v0.5.0.\n" +
				"Release page: https://github.com/jameskbb/casebook/releases/tag/v0.5.0\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := filepath.Join(t.TempDir(), "casebook.db")
			r, calls := runUpdateCLI(t, func(context.Context, update.Options) (update.Result, error) {
				return tc.res, nil
			}, "update", "--db", db)
			wantCode(t, r, 0)
			if r.Stdout != tc.want {
				t.Fatalf("stdout =\n%q\nwant\n%q", r.Stdout, tc.want)
			}
			if r.Stderr != "" {
				t.Fatalf("stderr = %q, want empty", r.Stderr)
			}
			if len(calls) != 1 {
				t.Fatalf("CheckUpdate called %d times, want 1", len(calls))
			}
			o := calls[0]
			if o.Current != Version || o.Flavor != update.FlavorCLI {
				t.Fatalf("options = %+v, want Current %q and the CLI flavor", o, Version)
			}
			if o.GOOS != "" || o.GOARCH != "" || o.BaseURL != "" || o.Client != nil {
				t.Fatalf("options = %+v, want defaults for GOOS, GOARCH, BaseURL and Client", o)
			}
			if _, err := os.Stat(db); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("update touched the database file: stat err = %v", err)
			}
		})
	}
}

func TestUpdateJSON(t *testing.T) {
	res := fixedResult(update.StateUpdateAvailable, testAsset)
	db := filepath.Join(t.TempDir(), "casebook.db")
	r, _ := runUpdateCLI(t, func(context.Context, update.Options) (update.Result, error) {
		return res, nil
	}, "update", "--json", "--db", db)
	wantCode(t, r, 0)
	var got map[string]any
	decodeJSON(t, r.Stdout, &got)
	want := map[string]any{
		"current":      "v0.4.0",
		"latest":       "v0.5.0",
		"state":        "update_available",
		"release_url":  "https://github.com/jameskbb/casebook/releases/tag/v0.5.0",
		"published_at": "2026-10-01T12:00:00Z",
		"checked_at":   "2026-10-14T09:30:00Z",
		"asset": map[string]any{
			"name":       testAsset.Name,
			"url":        testAsset.URL,
			"size_bytes": float64(12345),
		},
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("JSON =\n%s\nwant\n%s", gotJSON, wantJSON)
	}

	// Without an asset the key is left out.
	r, _ = runUpdateCLI(t, func(context.Context, update.Options) (update.Result, error) {
		return fixedResult(update.StateUpToDate, nil), nil
	}, "update", "--json", "--db", db)
	wantCode(t, r, 0)
	got = nil
	decodeJSON(t, r.Stdout, &got)
	if _, ok := got["asset"]; ok {
		t.Fatalf("JSON has an asset key with no asset: %s", r.Stdout)
	}
	if _, err := os.Stat(db); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("update --json touched the database file: stat err = %v", err)
	}
}

func TestUpdateErrors(t *testing.T) {
	for _, sentinel := range []error{update.ErrNoRelease, update.ErrUnavailable, update.ErrRateLimited} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s json=%v", sentinel, asJSON), func(t *testing.T) {
				db := filepath.Join(t.TempDir(), "casebook.db")
				wrapped := fmt.Errorf("%w (the repository may be private or have no release yet)", sentinel)
				args := []string{"update", "--db", db}
				if asJSON {
					args = append(args, "--json")
				}
				r, _ := runUpdateCLI(t, func(context.Context, update.Options) (update.Result, error) {
					return update.Result{}, wrapped
				}, args...)
				wantCode(t, r, 1)
				if r.Stdout != "" {
					t.Fatalf("stdout = %q, want empty", r.Stdout)
				}
				want := "casebook: " + wrapped.Error() + "\n"
				if r.Stderr != want {
					t.Fatalf("stderr = %q, want %q", r.Stderr, want)
				}
				if _, err := os.Stat(db); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("update touched the database file: stat err = %v", err)
				}
			})
		}
	}
}

func TestUpdateRejectsArguments(t *testing.T) {
	r, calls := runUpdateCLI(t, func(context.Context, update.Options) (update.Result, error) {
		t.Fatal("CheckUpdate ran for a bad command line")
		return update.Result{}, nil
	}, "update", "now")
	wantCode(t, r, 2)
	if len(calls) != 0 {
		t.Fatalf("CheckUpdate called %d times, want 0", len(calls))
	}
}

func TestUpdateHelp(t *testing.T) {
	r, calls := runUpdateCLI(t, func(context.Context, update.Options) (update.Result, error) {
		t.Fatal("CheckUpdate ran for --help")
		return update.Result{}, nil
	}, "update", "--help")
	wantCode(t, r, 0)
	if len(calls) != 0 {
		t.Fatalf("CheckUpdate called %d times for --help", len(calls))
	}
	wantContains(t, r.Stdout, "GitHub", "never downloads", "docs/updating.md", "docs/privacy.md")
}

func TestUpdateListedInHelp(t *testing.T) {
	r := runCLI(t, "", "--help")
	wantContains(t, r.Stdout, "update", "Check GitHub for a newer Casebook release")
}

func TestNewEnvChecksUpdatesWithUpdateCheck(t *testing.T) {
	env := NewEnv(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if env.CheckUpdate == nil {
		t.Fatal("NewEnv left CheckUpdate nil")
	}
}
