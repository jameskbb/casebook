package doctor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPTimeout bounds the whole MCP startup check: spawning the server, the
// initialize handshake and tools/list (D-14).
const MCPTimeout = 10 * time.Second

// mcpProtocolVersion is the initialize protocol version the check requests:
// the one Claude Code and Codex send today.
const mcpProtocolVersion = "2025-06-18"

// maxStderrBytes caps how much of the child's stderr is kept (the tail).
const maxStderrBytes = 64 << 10

// maxStderrLine caps the quoted server error line.
const maxStderrLine = 200

// MCPResult is what a successful MCP startup check learned.
type MCPResult struct {
	ServerName    string `json:"server"`
	ServerVersion string `json:"server_version"`
	Tools         int    `json:"tools"`
	// TempDB is true when dbPath did not exist and the check ran against a
	// temporary database instead (the real file is never created).
	TempDB bool `json:"temp_db"`
}

// MCPStartup spawns `<cliPath> mcp --db <dbPath>`, completes a real MCP
// initialize handshake (protocol 2025-06-18) and tools/list within timeout,
// and closes the session. When dbPath does not exist the server runs against
// a temporary database that is removed afterwards. The child's stderr is
// kept in a bounded buffer and only its last non-empty line (casebook's own
// one-line error, which never holds record text) is quoted in an error.
func MCPStartup(ctx context.Context, cliPath, dbPath string, timeout time.Duration) (MCPResult, error) {
	var res MCPResult
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		tmp, err := os.MkdirTemp("", "casebook-doctor-")
		if err != nil {
			return res, fmt.Errorf("MCP handshake failed: create a temporary database directory: %w", err)
		}
		defer os.RemoveAll(tmp)
		dbPath = filepath.Join(tmp, "casebook.db")
		res.TempDB = true
	}
	if timeout <= 0 {
		timeout = MCPTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cliPath, "mcp", "--db", dbPath)
	cmd.Env = childEnv(os.Environ())
	stderr := &tailBuffer{max: maxStderrBytes}
	cmd.Stderr = stderr
	cmd.WaitDelay = time.Second

	fail := func(err error) (MCPResult, error) {
		// Make sure the child is gone and its stderr fully copied before
		// reading the last line. Wait reports an error when the session
		// already waited; that is fine.
		cancel()
		_ = cmd.Wait()
		msg := "MCP handshake failed: " + err.Error()
		if line := stderr.lastLine(); line != "" {
			msg += "; server said: " + line
		}
		return MCPResult{TempDB: res.TempDB}, errors.New(msg)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "casebook-doctor", Version: "1"}, nil)
	cs, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd, TerminateDuration: 2 * time.Second},
		&mcp.ClientSessionOptions{ProtocolVersion: mcpProtocolVersion})
	if err != nil {
		return fail(err)
	}
	if init := cs.InitializeResult(); init != nil && init.ServerInfo != nil {
		res.ServerName = init.ServerInfo.Name
		res.ServerVersion = init.ServerInfo.Version
	}
	n := 0
	for _, err := range cs.Tools(ctx, nil) {
		if err != nil {
			_ = cs.Close()
			return fail(fmt.Errorf("tools/list: %w", err))
		}
		n++
	}
	res.Tools = n
	if err := cs.Close(); err != nil {
		return fail(fmt.Errorf("the server did not exit cleanly: %w", err))
	}
	return res, nil
}

// childEnv is environ without CASEBOOK_CLIENT (the check must not pose as
// another client) and with CASEBOOK_LOG_LEVEL=error, so the child's stderr
// holds errors only.
func childEnv(environ []string) []string {
	out := make([]string, 0, len(environ)+1)
	for _, kv := range environ {
		k, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(k) {
		case "CASEBOOK_CLIENT", "CASEBOOK_LOG_LEVEL":
			continue
		}
		out = append(out, kv)
	}
	return append(out, "CASEBOOK_LOG_LEVEL=error")
}

// tailBuffer keeps the last max bytes written to it. It is safe for the
// exec copy goroutine to write while another goroutine reads.
type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - b.max; over > 0 {
		b.buf = append(b.buf[:0:0], b.buf[over:]...)
	}
	return len(p), nil
}

// lastLine is the last non-empty line, trimmed and cut to maxStderrLine
// bytes (on a UTF-8 boundary).
func (b *tailBuffer) lastLine() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	lines := bytes.Split(b.buf, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(string(lines[i]))
		if line == "" {
			continue
		}
		if len(line) > maxStderrLine {
			cut := maxStderrLine
			for cut > 0 && !utf8Start(line[cut]) {
				cut--
			}
			line = line[:cut]
		}
		return line
	}
	return ""
}

// utf8Start reports whether c begins a UTF-8 sequence.
func utf8Start(c byte) bool { return c&0xC0 != 0x80 }
