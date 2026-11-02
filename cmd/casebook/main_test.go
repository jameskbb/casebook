package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/testenv"
)

// testVersion is stamped into the test binary through -ldflags.
const testVersion = "v0.0.0-test"

var (
	// binPath is the casebook binary built once by TestMain.
	binPath string
	// skipReason is set when the binary cannot be built for lack of a Go
	// toolchain; subprocess tests skip with it.
	skipReason string
)

// TestMain builds the casebook binary once for every subprocess test in this
// package (D-23).
func TestMain(m *testing.M) {
	testenv.CanonicalTempDir()
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	goTool, err := exec.LookPath("go")
	if err != nil {
		skipReason = "no Go toolchain on PATH; subprocess tests skipped"
		return m.Run()
	}
	dir, err := os.MkdirTemp("", "casebook-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "casebook test: temp dir:", err)
		return 1
	}
	defer os.RemoveAll(dir)
	name := "casebook"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(dir, name)
	build := exec.Command(goTool, "build",
		"-ldflags", "-X github.com/jameskbb/casebook/internal/version.Version="+testVersion,
		"-o", bin, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "casebook test: build failed: %v\n%s", err, out)
		return 1
	}
	binPath = bin
	return m.Run()
}

// binary returns the built casebook binary or skips the test.
func binary(t *testing.T) string {
	t.Helper()
	if binPath == "" {
		t.Skip(skipReason)
	}
	return binPath
}

// childEnv is the environment for a child casebook: the current environment
// without any CASEBOOK_ variable, CASEBOOK_HOME pointing at a fresh temp
// directory (so nothing touches the developer's data), plus extra.
func childEnv(t *testing.T, extra ...string) []string {
	t.Helper()
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(strings.ToUpper(kv), "CASEBOOK_") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "CASEBOOK_HOME="+t.TempDir())
	return append(env, extra...)
}

// lockedBuffer is a bytes.Buffer safe to write from the exec copy goroutine
// while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// connectMCP starts `casebook mcp --db dbPath` and connects an MCP client
// named clientName to it. protocol forces a protocol version when non-empty
// ("2025-06-18" is the initialize handshake real clients use today). The
// returned buffer collects the child's stderr. Cleanup closes the session
// (closing stdin and waiting for the child) and fails the test when the
// child did not exit cleanly.
func connectMCP(t *testing.T, dbPath, clientName, protocol string, extraEnv ...string) (*mcp.ClientSession, *lockedBuffer) {
	t.Helper()
	cmd := exec.Command(binary(t), "mcp", "--db", dbPath)
	cmd.Env = childEnv(t, extraEnv...)
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	client := mcp.NewClient(&mcp.Implementation{Name: clientName, Version: "test"}, nil)
	var opts *mcp.ClientSessionOptions
	if protocol != "" {
		opts = &mcp.ClientSessionOptions{ProtocolVersion: protocol}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cs, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, opts)
	if err != nil {
		t.Fatalf("connect to casebook mcp: %v\nstderr:\n%s", err, stderr.String())
	}
	var once sync.Once
	t.Cleanup(func() {
		once.Do(func() {
			if err := cs.Close(); err != nil {
				t.Errorf("close casebook mcp session: %v\nstderr:\n%s", err, stderr.String())
			}
		})
	})
	return cs, stderr
}
