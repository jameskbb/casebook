package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// Per-directory active investigations (ADR-0015). A directory, usually a git
// repository root, can be bound to an investigation. An App opened with
// WithWorkDir resolves "the active investigation" as the binding of its
// working directory or of the nearest bound ancestor, and falls back to the
// global active investigation (ADR-0008) when no binding applies. An App
// without a working directory (the desktop app) sees only the global one.

// Active scopes reported by ActiveInvestigationInfo.
const (
	// ActiveScopeDirectory: the active investigation comes from a directory
	// binding.
	ActiveScopeDirectory = "directory"
	// ActiveScopeGlobal: the active investigation is the global setting.
	ActiveScopeGlobal = "global"
)

// maxBindingDirBytes bounds a bound directory path.
const maxBindingDirBytes = 4096

// WithWorkDir sets the directory a surface works in: the CLI's current
// directory, a hook payload's cwd, the MCP server's working directory. An
// empty dir (the default) means only the global active investigation
// applies. The path is made absolute and cleaned; symlinks are resolved when
// possible.
func WithWorkDir(dir string) Option {
	return func(o *options) { o.workDir = dir }
}

// ActiveInfo is the active investigation as this App resolves it.
type ActiveInfo struct {
	// Investigation is the effective active investigation, nil when none.
	Investigation *Investigation `json:"investigation"`
	// Scope is ActiveScopeDirectory, ActiveScopeGlobal, or "" when none.
	Scope string `json:"scope"`
	// Dir is the bound directory when Scope is directory, else "".
	Dir string `json:"dir"`
	// GlobalRef is the global active investigation's ref, "" when none.
	GlobalRef string `json:"global_ref"`
}

// DirectoryBinding is one directory bound to an investigation.
type DirectoryBinding struct {
	Dir              string `json:"dir"`
	InvestigationRef string `json:"investigation_ref"`
	Title            string `json:"title"`
	UpdatedAt        string `json:"updated_at"`
}

// ProjectRoot returns the directory a binding made from dir uses: the
// nearest of dir and its ancestors that holds a .git entry (a repository
// root or a worktree), else dir itself. dir is made absolute and cleaned
// first; symlinks are resolved when possible.
func ProjectRoot(dir string) (string, error) {
	d, err := cleanDir(dir)
	if err != nil {
		return "", err
	}
	for cur := d; ; {
		if _, err := os.Lstat(filepath.Join(cur, ".git")); err == nil {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return d, nil
		}
		cur = parent
	}
}

// cleanDir makes dir absolute and clean, resolving symlinks when it can.
func cleanDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", domain.NewValidationError("dir", "is required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", domain.NewValidationError("dir", "cannot be made absolute")
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		abs = r
	}
	return filepath.Clean(abs), nil
}

// foldDirCase reports whether directory bindings match case-insensitively:
// true on Windows, whose file names are case-insensitive. A binding keeps
// the spelling it was made with (the on-disk case when the directory exists,
// since cleanDir resolves it), so users see their paths unmangled.
var foldDirCase = runtime.GOOS == "windows"

// sameDir reports whether two cleaned directories name the same binding.
func sameDir(a, b string) bool {
	if foldDirCase {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// nearestBinding returns the binding of the first of dirs (nearest first)
// that has one, matching with sameDir. The table holds one row per bound
// directory, so reading it whole is cheap.
func nearestBinding(ctx context.Context, q store.Querier, dirs []string) (store.DirectoryBinding, bool, error) {
	if len(dirs) == 0 {
		return store.DirectoryBinding{}, false, nil
	}
	rows, err := store.ListDirectoryBindings(ctx, q)
	if err != nil {
		return store.DirectoryBinding{}, false, err
	}
	for _, d := range dirs {
		for _, b := range rows {
			if sameDir(b.Dir, d) {
				return b, true, nil
			}
		}
	}
	return store.DirectoryBinding{}, false, nil
}

// lookupDirs returns dir and every ancestor, cleaned, nearest first, or
// nil when dir is "" or cannot be cleaned.
func lookupDirs(dir string) []string {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	d, err := cleanDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for cur := d; ; {
		out = append(out, cur)
		parent := filepath.Dir(cur)
		if parent == cur {
			return out
		}
		cur = parent
	}
}

// activeID is the effective active investigation id: the nearest directory
// binding of the App's working directory, else the global setting. binding
// is set when a directory binding applied.
func (a *App) activeID(ctx context.Context, q store.Querier) (id int64, ok bool, binding *store.DirectoryBinding, err error) {
	if len(a.workDirs) > 0 {
		b, found, err := nearestBinding(ctx, q, a.workDirs)
		if err != nil {
			return 0, false, nil, err
		}
		if found {
			return b.InvestigationID, true, &b, nil
		}
	}
	id, ok, err = store.GetActiveInvestigationID(ctx, q)
	return id, ok, nil, err
}

// markActive sets inv.IsActive from the effective active investigation when
// the App has a working directory (the store computes the global flag).
func (a *App) markActive(ctx context.Context, q store.Querier, inv *domain.Investigation) error {
	if len(a.workDirs) == 0 {
		return nil
	}
	id, ok, _, err := a.activeID(ctx, q)
	if err != nil {
		return err
	}
	inv.IsActive = ok && id == inv.ID
	return nil
}

// getInvestigationByID reads investigation id with IsActive set for this
// App (markActive).
func (a *App) getInvestigationByID(ctx context.Context, q store.Querier, id int64) (domain.Investigation, error) {
	inv, err := store.GetInvestigationByID(ctx, q, id)
	if err != nil {
		return inv, err
	}
	if err := a.markConclusionAttention(ctx, q, &inv); err != nil {
		return inv, err
	}
	return inv, a.markActive(ctx, q, &inv)
}

// ActiveInvestigationInfo reports the effective active investigation, where
// it comes from (a directory binding or the global setting) and the global
// one.
func (a *App) ActiveInvestigationInfo(ctx context.Context) (ActiveInfo, error) {
	var out ActiveInfo
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		gid, gok, err := store.GetActiveInvestigationID(ctx, tx)
		if err != nil {
			return err
		}
		if gok {
			inv, err := store.GetInvestigationByID(ctx, tx, gid)
			switch {
			case err == nil:
				out.GlobalRef = inv.Ref
			case !errors.Is(err, sql.ErrNoRows):
				return err
			}
		}
		id, ok, binding, err := a.activeID(ctx, tx)
		if err != nil || !ok {
			return err
		}
		inv, err := a.getInvestigationByID(ctx, tx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		out.Investigation = &inv
		out.Scope = ActiveScopeGlobal
		if binding != nil {
			out.Scope = ActiveScopeDirectory
			out.Dir = binding.Dir
		}
		return nil
	})
	if err != nil {
		return ActiveInfo{}, a.fail("active.get", err)
	}
	return out, nil
}

// BindDirectory binds dir to invRef, so surfaces working in dir or below it
// use invRef as their active investigation. A non-empty dir is bound as
// given (made absolute and cleaned); an empty dir means the project root
// (ProjectRoot) of the App's working directory, a ValidationError on dir
// when the App has none. Binding a directory to the investigation it is
// already bound to is a no-op with no activity; otherwise one active.bind
// row is recorded on the investigation. invRef is required.
func (a *App) BindDirectory(ctx context.Context, actor Actor, dir, invRef string) (DirectoryBinding, error) {
	const op = "active.bind"
	actor, err := actor.Validate()
	if err != nil {
		return DirectoryBinding{}, a.fail(op, err)
	}
	if strings.TrimSpace(invRef) == "" {
		return DirectoryBinding{}, a.fail(op, domain.NewValidationError("investigation_ref", "is required"))
	}
	key, err := a.bindingDir(dir)
	if err != nil {
		return DirectoryBinding{}, a.fail(op, err)
	}
	var out DirectoryBinding
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		inv, err := a.resolveInvestigation(ctx, tx, invRef)
		if err != nil {
			return err
		}
		cur, found, err := nearestBinding(ctx, tx, []string{key})
		if err != nil {
			return err
		}
		out = DirectoryBinding{Dir: key, InvestigationRef: inv.Ref, Title: inv.Title, UpdatedAt: now}
		if found && cur.InvestigationID == inv.ID {
			out.Dir = cur.Dir
			out.UpdatedAt = cur.UpdatedAt
			return nil
		}
		if found && cur.Dir != key {
			// The same directory spelled differently (Windows): keep one
			// row, under the new spelling.
			if _, err := store.DeleteDirectoryBinding(ctx, tx, cur.Dir); err != nil {
				return err
			}
		}
		if err := store.SetDirectoryBinding(ctx, tx, key, inv.ID, now); err != nil {
			return err
		}
		return a.recordActivity(ctx, tx, actor, inv.ID, op, inv.Ref,
			fmt.Sprintf("Made %s the active investigation for %s", inv.Ref, key), "", now)
	})
	if err != nil {
		return DirectoryBinding{}, a.fail(op, err)
	}
	a.log.Debug(op, "ref", out.InvestigationRef)
	return out, nil
}

// UnbindDirectory removes a directory binding. An empty dir means the
// binding that applies to the App's working directory (its own or the
// nearest bound ancestor's); a non-empty dir names the bound directory
// exactly (after cleaning). It reports the directory it unbound, or "" when
// there was nothing to remove (no activity then). One global active.unbind
// row is recorded.
func (a *App) UnbindDirectory(ctx context.Context, actor Actor, dir string) (string, error) {
	const op = "active.unbind"
	actor, err := actor.Validate()
	if err != nil {
		return "", a.fail(op, err)
	}
	var candidates []string
	if strings.TrimSpace(dir) == "" {
		if len(a.workDirs) == 0 {
			return "", a.fail(op, domain.NewValidationError("dir", "is required without a working directory"))
		}
		candidates = a.workDirs
	} else {
		d, err := cleanDir(dir)
		if err != nil {
			return "", a.fail(op, err)
		}
		candidates = []string{d}
	}
	removed := ""
	err = a.db.WithWriteTx(ctx, func(tx *sql.Tx) error {
		now := a.timestamp()
		b, found, err := nearestBinding(ctx, tx, candidates)
		if err != nil || !found {
			return err
		}
		ref := ""
		if inv, err := store.GetInvestigationByID(ctx, tx, b.InvestigationID); err == nil {
			ref = inv.Ref
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := store.DeleteDirectoryBinding(ctx, tx, b.Dir); err != nil {
			return err
		}
		removed = b.Dir
		summary := "Removed the active investigation for " + b.Dir
		if ref != "" {
			summary += " (was " + ref + ")"
		}
		return a.recordActivity(ctx, tx, actor, 0, op, ref, summary, "", now)
	})
	if err != nil {
		return "", a.fail(op, err)
	}
	return removed, nil
}

// ListDirectoryBindings returns every directory binding ordered by
// directory. The slice is never nil.
func (a *App) ListDirectoryBindings(ctx context.Context) ([]DirectoryBinding, error) {
	out := []DirectoryBinding{}
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		rows, err := store.ListDirectoryBindings(ctx, tx)
		if err != nil {
			return err
		}
		for _, b := range rows {
			inv, err := store.GetInvestigationByID(ctx, tx, b.InvestigationID)
			if err != nil {
				return err
			}
			out = append(out, DirectoryBinding{Dir: b.Dir, InvestigationRef: inv.Ref, Title: inv.Title, UpdatedAt: b.UpdatedAt})
		}
		return nil
	})
	if err != nil {
		return nil, a.fail("active.bindings", err)
	}
	return out, nil
}

// WorkDir returns the cleaned working directory the App was opened with, ""
// when none.
func (a *App) WorkDir() string { return a.workDir }

// bindingDir is the directory a binding of dir stores: dir itself, cleaned,
// or with "" the project root of the App's working directory.
func (a *App) bindingDir(dir string) (string, error) {
	var (
		root string
		err  error
	)
	if strings.TrimSpace(dir) == "" {
		if a.workDir == "" {
			return "", domain.NewValidationError("dir", "is required without a working directory")
		}
		root, err = ProjectRoot(a.workDir)
	} else {
		root, err = cleanDir(dir)
	}
	if err != nil {
		return "", err
	}
	if len(root) > maxBindingDirBytes {
		return "", domain.NewValidationError("dir", "must be at most %d bytes", maxBindingDirBytes)
	}
	return root, nil
}
