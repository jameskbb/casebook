package app_test

// Package boundary enforcement (D-03, APP-01). Every later phase is checked
// automatically: a surface that reaches past internal/app fails go test.

import (
	"go/parser"
	gotoken "go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/jameskbb/casebook"

// Internal packages with import rules.
const (
	pkgApp         = modulePath + "/internal/app"
	pkgStore       = modulePath + "/internal/store"
	pkgDomain      = modulePath + "/internal/domain"
	pkgContextpack = modulePath + "/internal/contextpack"
	pkgDeps        = modulePath + "/internal/deps"
	pkgUpdate      = modulePath + "/internal/update"
)

// isStdlib reports whether an import path is from the standard library (no
// dot in its first element).
func isStdlib(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

// within reports whether the slash-separated directory dir is pkgDir or
// below it.
func within(dir, pkgDir string) bool {
	return dir == pkgDir || strings.HasPrefix(dir, pkgDir+"/")
}

// boundaryViolations applies the import rules to one file. rel is the
// file's slash-separated path relative to the module root.
//
//   - Only internal/app itself (not internal/app/apptest) and internal/store
//     may import internal/store.
//   - Only internal/app, internal/store and internal/contextpack may import
//     internal/domain, and only internal/app may import internal/contextpack:
//     surfaces use the aliases internal/app re-exports.
//   - Non-test files in internal/domain and internal/paths import only the
//     standard library.
//   - Non-test files in internal/contextpack import only the standard library
//     and internal/domain.
//   - Nothing imports internal/deps (it only pins go.mod requirements).
//   - Only internal/cli and desktop (and internal/update itself) may import
//     internal/update, the manual update check (ADR-0022): internal/app,
//     internal/mcpserver, internal/integrations (hooks) and internal/doctor
//     never reach it, so `casebook mcp` has no network code path.
//   - No non-test file in the module imports a network package: net and
//     everything below it except net/url, crypto/tls, and the network
//     modules (golang.org/x/net, golang.org/x/oauth2, websocket, gRPC and
//     HTTP framework packages). The one exception is net/http in the
//     non-test files of internal/update, the single GET a person asks for
//     when checking for updates (ADR-0022); everything else there stays
//     forbidden too. A person's AI client handles any external reads and
//     writes.
//
// A package's own files (including its external _test package) may always
// import that package.
func boundaryViolations(rel string, imports []string) []string {
	dir := filepath.ToSlash(filepath.Dir(rel))
	isTest := strings.HasSuffix(rel, "_test.go")
	var out []string
	bad := func(imp, why string) { out = append(out, rel+": imports "+imp+" ("+why+")") }
	for _, imp := range imports {
		switch {
		case imp == pkgStore && dir != "internal/app" && !within(dir, "internal/store"):
			bad(imp, "only internal/app may use internal/store; surfaces call internal/app")
		case imp == pkgDomain && dir != "internal/app" && !within(dir, "internal/store") &&
			!within(dir, "internal/contextpack") && !within(dir, "internal/domain"):
			bad(imp, "surfaces use the internal/app aliases, not internal/domain")
		case imp == pkgContextpack && dir != "internal/app" && !within(dir, "internal/contextpack"):
			bad(imp, "surfaces use app.ContextPack and app.RenderContext, not internal/contextpack")
		case imp == pkgDeps || strings.HasPrefix(imp, pkgDeps+"/"):
			bad(imp, "internal/deps only pins go.mod requirements")
		case (imp == pkgUpdate || strings.HasPrefix(imp, pkgUpdate+"/")) && !within(dir, "internal/cli") &&
			!within(dir, "desktop") && !within(dir, "internal/update"):
			bad(imp, "only internal/cli and desktop may use internal/update (ADR-0022); the engine, MCP server, hooks and doctor stay network-free")
		}
		if isTest {
			continue
		}
		switch {
		case (within(dir, "internal/domain") || within(dir, "internal/paths")) && !isStdlib(imp):
			bad(imp, "internal/domain and internal/paths use the standard library only")
		case within(dir, "internal/contextpack") && !isStdlib(imp) && imp != pkgDomain:
			bad(imp, "internal/contextpack uses the standard library and internal/domain only")
		case imp == "net/http" && within(dir, "internal/update"):
			// The update check's one GET (ADR-0022).
		case isNetworkPackage(imp):
			bad(imp, "Casebook makes no network connections outside internal/update (ADR-0020, ADR-0022); only net/url may be imported")
		}
	}
	return out
}

// networkModules are module paths whose packages open network connections
// or serve network protocols.
var networkModules = []string{
	"golang.org/x/net", "golang.org/x/oauth2", "github.com/gorilla/websocket", "nhooyr.io/websocket",
	"github.com/coder/websocket", "google.golang.org/grpc", "github.com/labstack/echo", "github.com/gin-gonic/gin",
	"github.com/valyala/fasthttp", "github.com/go-resty/resty", "github.com/hashicorp/go-retryablehttp",
	"github.com/modelcontextprotocol/go-sdk/auth", "github.com/modelcontextprotocol/go-sdk/oauthex",
}

// isNetworkPackage reports whether imp opens or serves network connections:
// net and every package below it except net/url (which only parses),
// crypto/tls, and the packages of networkModules.
func isNetworkPackage(imp string) bool {
	if imp == "net" || (strings.HasPrefix(imp, "net/") && imp != "net/url") || imp == "crypto/tls" {
		return true
	}
	for _, m := range networkModules {
		if imp == m || strings.HasPrefix(imp, m+"/") {
			return true
		}
	}
	return false
}

// TestPackageBoundaries parses the imports of every .go file in the module
// (skipping .git, .planning, node_modules and frontend directories) and
// reports every boundary violation with its file and import path.
func TestPackageBoundaries(t *testing.T) {
	root := moduleRoot(t)
	fset := gotoken.NewFileSet()
	files := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".planning", ".claude", "node_modules", "frontend":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			return nil
		}
		var imports []string
		for _, spec := range f.Imports {
			p, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Errorf("%s: bad import %s", rel, spec.Path.Value)
				continue
			}
			imports = append(imports, p)
		}
		for _, v := range boundaryViolations(rel, imports) {
			t.Error(v)
		}
		files++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 50 {
		t.Fatalf("scanned only %d Go files; the module walk is broken", files)
	}
}

// TestBoundaryRules proves the rules catch the violations they exist for.
func TestBoundaryRules(t *testing.T) {
	cases := []struct {
		rel     string
		imports []string
		want    bool
	}{
		{"internal/cli/root.go", []string{pkgStore}, true},
		{"internal/mcpserver/tools_test.go", []string{pkgStore}, true},
		{"internal/app/apptest/apptest.go", []string{pkgStore}, true},
		{"desktop/main.go", []string{pkgDomain}, true},
		{"cmd/casebook/main.go", []string{pkgContextpack}, true},
		{"internal/domain/enums.go", []string{"github.com/google/uuid"}, true},
		{"internal/paths/paths.go", []string{pkgDomain}, true},
		{"internal/contextpack/build.go", []string{pkgApp}, true},
		{"internal/store/store.go", []string{pkgDeps}, true},
		{"internal/app/app.go", []string{pkgStore, pkgDomain, pkgContextpack, "context"}, false},
		{"internal/store/tx.go", []string{pkgDomain, "modernc.org/sqlite"}, false},
		{"internal/contextpack/build.go", []string{pkgDomain, "strings"}, false},
		{"internal/domain/enums_test.go", []string{pkgDomain, "testing"}, false},
		{"internal/cli/root.go", []string{pkgApp, modulePath + "/internal/paths", "github.com/spf13/cobra"}, false},
		// No network package outside tests, anywhere (ADR-0020).
		{"internal/mcpserver/server.go", []string{"net/http"}, true},
		{"internal/doctor/checks.go", []string{"net"}, true},
		{"internal/app/app.go", []string{"crypto/tls"}, true},
		{"desktop/main.go", []string{"net/http/httptest"}, true},
		{"desktop/tools/aiwrite/main.go", []string{"net"}, true},
		{"scripts/readme-example/main.go", []string{"net/http"}, true},
		{"internal/cli/root.go", []string{"golang.org/x/net/http2"}, true},
		{"internal/cli/root.go", []string{"golang.org/x/oauth2"}, true},
		{"internal/cli/root.go", []string{"net/netip"}, true},
		{"internal/cli/root.go", []string{"net/smtp"}, true},
		{"internal/mcpserver/server.go", []string{"github.com/modelcontextprotocol/go-sdk/auth"}, true},
		{"internal/store/store.go", []string{"net/url"}, false},
		{"internal/mcpserver/server.go", []string{"github.com/modelcontextprotocol/go-sdk/mcp"}, false},
		{"internal/mcpserver/server_test.go", []string{"net/http"}, false},
		// internal/update: the one network package, and only net/http (ADR-0022).
		{"internal/update/update.go", []string{"net/http"}, false},
		{"internal/update/update.go", []string{"net"}, true},
		{"internal/update/update.go", []string{"crypto/tls"}, true},
		{"internal/update/update.go", []string{"net/http/httptest"}, true},
		{"internal/update/update.go", []string{"golang.org/x/net/http2"}, true},
		{"internal/update/update_test.go", []string{"net", "net/http/httptest"}, false},
		{"internal/cli/update.go", []string{"net/http"}, true},
		{"desktop/update.go", []string{"net/http"}, true},
		// Only internal/cli and desktop may use internal/update.
		{"internal/cli/update.go", []string{pkgUpdate}, false},
		{"desktop/update.go", []string{pkgUpdate}, false},
		{"internal/update/update_test.go", []string{pkgUpdate}, false},
		{"internal/app/update.go", []string{pkgUpdate}, true},
		{"internal/mcpserver/x.go", []string{pkgUpdate}, true},
		{"internal/integrations/x.go", []string{pkgUpdate}, true},
		{"internal/doctor/x.go", []string{pkgUpdate}, true},
		{"cmd/casebook/main.go", []string{pkgUpdate}, true},
		{"internal/app/update_test.go", []string{pkgUpdate}, true},
	}
	for _, c := range cases {
		got := boundaryViolations(c.rel, c.imports)
		if (len(got) > 0) != c.want {
			t.Errorf("%s importing %v: violations %v, want violation %v", c.rel, c.imports, got, c.want)
		}
	}
}
