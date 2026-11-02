package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Claude Code user-scope config keys (D-02).
const (
	claudeServersKey  = "mcpServers"
	claudeServerName  = "casebook"
	claudeServerEntry = claudeServersKey + "." + claudeServerName
)

// ClaudeOptions configures the Claude Code setup functions.
type ClaudeOptions struct {
	// Paths locates the config files (from ResolvePaths).
	Paths Paths
	// DBPath, when set, is an absolute database path; the server's args
	// become ["mcp", "--db", DBPath].
	DBPath string
	// Now is the backup timestamp clock; nil means time.Now.
	Now func() time.Time
	// Hooks also installs the Casebook hooks in Paths.ClaudeSettings
	// (setup claude --hooks).
	Hooks bool
	// GOOS picks the form of hook handlers ("windows": exec form, anything
	// else: shell form quoted for sh); "" means runtime.GOOS.
	GOOS string
	// HookScope is the hook scope written into the hook handlers (setup
	// claude --hooks --scope): ScopeMarked adds "--scope marked" to each
	// handler; "" or ScopeAll adds nothing, leaving the default and
	// CASEBOOK_HOOK_SCOPE in charge.
	HookScope string
	// HookQuiet writes --quiet-when-inactive into the session-start hook
	// handler (setup claude --hooks --quiet-when-inactive), so a session
	// with no active investigation gets nothing instead of the one-line
	// hint to offer a case.
	HookQuiet bool
}

// now returns the current time from opts.Now or time.Now.
func (o ClaudeOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// claudeServer is the Claude Code stdio MCP server entry, with fields in the
// order `claude mcp add -s user` writes them.
type claudeServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// mcpArgs are the casebook arguments that start the MCP server.
func mcpArgs(dbPath string) []string {
	if dbPath != "" {
		return []string{"mcp", "--db", dbPath}
	}
	return []string{"mcp"}
}

// ClaudeMCPServer returns the mcpServers.casebook entry as compact JSON:
// {"type":"stdio","command":<cliPath>,"args":["mcp"],"env":{}}, with
// "--db", <DBPath> appended to args when opts.DBPath is set (D-02, D-03).
func ClaudeMCPServer(cliPath string, opts ClaudeOptions) json.RawMessage {
	raw, err := marshalNoEscape(claudeServer{
		Type:    "stdio",
		Command: cliPath,
		Args:    mcpArgs(opts.DBPath),
		Env:     map[string]string{},
	})
	if err != nil {
		// Strings, a string slice and an empty map always encode.
		panic("integrations: encode Claude MCP server: " + err.Error())
	}
	return raw
}

// claudeDoc is a parsed .claude.json with its mcpServers object.
type claudeDoc struct {
	file    configFile
	root    *jsonObject
	servers *jsonObject // nil when the document has no mcpServers
}

// readClaudeDoc reads and parses the Claude Code user config, refusing
// anything Casebook should not change: invalid JSON, a non-object
// document, duplicate keys at the top level or in mcpServers, an mcpServers
// that is not an object, or an mcpServers.casebook that is not an object.
func readClaudeDoc(path string) (claudeDoc, error) {
	f, err := readConfigFile(path)
	if err != nil {
		return claudeDoc{}, err
	}
	d := claudeDoc{file: f}
	root, err := parseJSONObject(f.Data)
	if err != nil {
		return claudeDoc{}, &RefusedError{File: f.Path, Reason: err.Error()}
	}
	d.root = root
	servers, ok, err := root.childObject(claudeServersKey)
	if err != nil {
		return claudeDoc{}, &RefusedError{File: f.Path, Reason: err.Error()}
	}
	if ok {
		d.servers = servers
		if v, ok := servers.Get(claudeServerName); ok && !isJSONObject(v) {
			return claudeDoc{}, &RefusedError{File: f.Path, Reason: claudeServerEntry + " is not a JSON object"}
		}
	}
	return d, nil
}

// isJSONObject reports whether the raw value is an object.
func isJSONObject(v json.RawMessage) bool {
	t := bytes.TrimLeft(v, jsonSpace)
	return len(t) > 0 && t[0] == '{'
}

// sameJSON reports whether two raw values have identical tokens.
func sameJSON(a, b json.RawMessage) bool {
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return false
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}

// save encodes the document with servers set back in place and commits it.
func (d claudeDoc) save(opts ClaudeOptions) (backup string, changed bool, err error) {
	srv, err := d.servers.Encode()
	if err != nil {
		return "", false, err
	}
	d.root.Set(claudeServersKey, bytes.TrimRight(srv, "\n"))
	data, err := d.root.Encode()
	if err != nil {
		return "", false, err
	}
	return d.file.commit(data, opts.now())
}

// SetupClaude registers the Casebook MCP server in Claude Code's user config
// (opts.Paths.ClaudeJSON) as mcpServers.casebook, touching no other key.
// An existing casebook entry is replaced in place; a new one is appended to
// mcpServers, which is appended to the document when missing. A run that
// changes nothing writes nothing and reports unchanged.
//
// With opts.Hooks it also merges the Casebook hooks into
// opts.Paths.ClaudeSettings (installHooks) and reports that file second.
// Both files are read and checked before either is written, so a refusal
// in one writes nothing in the other (INT-11). When .claude.json was written
// but writing settings.json then fails, the changes made so far are
// returned with an error naming that file, its action and its backup
// (secondWriteError).
func SetupClaude(cliPath string, opts ClaudeOptions) ([]FileChange, error) {
	d, err := readClaudeDoc(opts.Paths.ClaudeJSON)
	if err != nil {
		return nil, err
	}
	var s *settingsEdit
	if opts.Hooks {
		s, err = planHookInstall(cliPath, opts)
		if err != nil {
			return nil, err
		}
	}
	change, err := setupMCP(d, cliPath, opts)
	if err != nil {
		return nil, err
	}
	changes := []FileChange{change}
	if s != nil {
		sc, err := s.save(opts)
		if err != nil {
			return secondWriteError(change, s.file.Path, err)
		}
		changes = append(changes, sc)
	}
	return changes, nil
}

// secondWriteError is the result of a two-file run whose second write
// failed after the first file's change (D-04: report the file, action and
// backup). When the first file was not written it is just err; otherwise
// the first change is returned too, and the error names it and its backup.
func secondWriteError(first FileChange, second string, err error) ([]FileChange, error) {
	if first.Action == ActionUnchanged {
		return nil, err
	}
	backup := "no backup: the file was new"
	if first.Backup != "" {
		backup = "backup " + first.Backup
	}
	return []FileChange{first}, fmt.Errorf("%s was %s (%s), but %s was not changed: %w",
		first.File, first.Action, backup, second, err)
}

// claudeManagedKeys are the members of mcpServers.casebook that setup
// owns. Every other member (env, timeout and so on) belongs to the user and
// is kept as it is (WR-04).
var claudeManagedKeys = []string{"type", "command", "args"}

// mergeClaudeServer returns the mcpServers.casebook entry for an existing
// entry old (an object): old with type, command and args set to Casebook's
// values in place and every other member kept, plus whether that changes
// any managed member, and the user's extra keys (env counts only when it is
// not empty).
func mergeClaudeServer(old json.RawMessage, cliPath string, opts ClaudeOptions) (merged json.RawMessage, changed bool, extra []string, err error) {
	entry, err := parseJSONObject(old)
	if err != nil {
		return nil, false, nil, err
	}
	desired, err := parseJSONObject(ClaudeMCPServer(cliPath, opts))
	if err != nil {
		return nil, false, nil, err
	}
	for _, k := range claudeManagedKeys {
		want, _ := desired.Get(k)
		if cur, ok := entry.Get(k); !ok || !sameJSON(cur, want) {
			entry.Set(k, want)
			changed = true
		}
	}
	for _, m := range entry.Members {
		switch {
		case slices.Contains(claudeManagedKeys, m.Key):
		case m.Key == "env" && sameJSON(m.Value, json.RawMessage(`{}`)):
		default:
			extra = append(extra, m.Key)
		}
	}
	enc, err := entry.Encode()
	if err != nil {
		return nil, false, nil, err
	}
	return json.RawMessage(bytes.TrimRight(enc, "\n")), changed, extra, nil
}

// setupMCP writes mcpServers.casebook into the parsed .claude.json. An
// existing entry keeps every member but type, command and args (WR-04), so
// a user's env (such as CASEBOOK_DB) or timeout survives a rerun.
func setupMCP(d claudeDoc, cliPath string, opts ClaudeOptions) (FileChange, error) {
	change := FileChange{File: d.file.Path, Entries: []string{claudeServerEntry}}
	if d.servers == nil {
		d.servers = &jsonObject{}
	}
	entry := ClaudeMCPServer(cliPath, opts)
	old, existed := d.servers.Get(claudeServerName)
	if existed {
		merged, changed, extra, err := mergeClaudeServer(old, cliPath, opts)
		if err != nil {
			return FileChange{}, &RefusedError{File: d.file.Path, Reason: claudeServerEntry + ": " + err.Error()}
		}
		if len(extra) > 0 {
			change.Note = "kept your " + strings.Join(extra, ", ") + " in " + claudeServerEntry
		}
		if !changed {
			// Already registered: leave the file alone, even its formatting.
			change.Action = ActionUnchanged
			return change, nil
		}
		entry = merged
	}
	d.servers.Set(claudeServerName, entry)
	backup, changed, err := d.save(opts)
	if err != nil {
		return FileChange{}, err
	}
	switch {
	case !changed:
		change.Action = ActionUnchanged
	case existed:
		change.Action = ActionUpdated
	default:
		change.Action = ActionAdded
	}
	change.Backup = backup
	return change, nil
}

// settingsEdit is a planned change to Claude Code's settings.json: the file
// as read, the edited document (nil when nothing changes) and the report.
type settingsEdit struct {
	file   configFile
	root   *jsonObject
	change FileChange
	// action is reported when the write changes the bytes.
	action Action
}

// readSettings reads and parses settings.json, refusing a file that does not
// parse as one JSON object.
func readSettings(path string) (configFile, *jsonObject, error) {
	f, err := readConfigFile(path)
	if err != nil {
		return f, nil, err
	}
	root, err := parseJSONObject(f.Data)
	if err != nil {
		return f, nil, &RefusedError{File: f.Path, Reason: err.Error()}
	}
	return f, root, nil
}

// planHookInstall reads settings.json and computes the hook install without
// writing anything.
func planHookInstall(cliPath string, opts ClaudeOptions) (*settingsEdit, error) {
	f, root, err := readSettings(opts.Paths.ClaudeSettings)
	if err != nil {
		return nil, err
	}
	hooks, err := hooksObject(root)
	if err != nil {
		return nil, &RefusedError{File: f.Path, Reason: err.Error()}
	}
	had := hasCasebookHandler(hooks)
	note, changed, err := installHooks(root, cliPath, opts)
	if err != nil {
		return nil, &RefusedError{File: f.Path, Reason: err.Error()}
	}
	e := &settingsEdit{
		file:   f,
		change: FileChange{File: f.Path, Action: ActionUnchanged, Entries: hookEntries(), Note: note},
		action: ActionUpdated,
	}
	if !had {
		e.action = ActionAdded
	}
	if changed {
		e.root = root
	}
	return e, nil
}

// save writes the planned settings.json change, if any, and returns the
// report.
func (e *settingsEdit) save(opts ClaudeOptions) (FileChange, error) {
	if e.root == nil {
		return e.change, nil
	}
	data, err := e.root.Encode()
	if err != nil {
		return FileChange{}, err
	}
	backup, changed, err := e.file.commit(data, opts.now())
	if err != nil {
		return FileChange{}, err
	}
	if changed {
		e.change.Action = e.action
		e.change.Backup = backup
	}
	return e.change, nil
}

// RemoveClaude deletes mcpServers.casebook from Claude Code's user config
// and every Casebook hook handler from settings.json, and nothing else
// (D-06, INT-03). The mcpServers object stays in place even when it becomes
// empty: Casebook cannot tell whether the user created it. Both files are
// read and checked before either is written. A missing file, or one with
// nothing of Casebook's, reports unchanged and is left alone (a missing file
// is never created). The changes are .claude.json first, then settings.json
// with the hook events that changed as its entries. When .claude.json was
// written but writing settings.json then fails, the .claude.json change is
// returned with an error naming it and its backup (secondWriteError).
func RemoveClaude(opts ClaudeOptions) ([]FileChange, error) {
	d, err := readClaudeDoc(opts.Paths.ClaudeJSON)
	if err != nil {
		return nil, err
	}
	sf, sroot, err := readSettings(opts.Paths.ClaudeSettings)
	if err != nil {
		return nil, err
	}
	events, err := removeHooks(sroot)
	if err != nil {
		return nil, &RefusedError{File: sf.Path, Reason: err.Error()}
	}

	change := FileChange{File: d.file.Path, Action: ActionUnchanged, Entries: []string{claudeServerEntry}}
	if d.servers != nil && d.servers.Delete(claudeServerName) {
		backup, changed, err := d.save(opts)
		if err != nil {
			return nil, err
		}
		if changed {
			change.Action = ActionRemoved
			change.Backup = backup
		}
	}

	sc := FileChange{File: sf.Path, Action: ActionUnchanged, Entries: []string{}}
	if len(events) > 0 {
		sc.Entries = events
		data, err := sroot.Encode()
		if err != nil {
			return secondWriteError(change, sf.Path, err)
		}
		backup, changed, err := sf.commit(data, opts.now())
		if err != nil {
			return secondWriteError(change, sf.Path, err)
		}
		if changed {
			sc.Action = ActionRemoved
			sc.Backup = backup
		}
	}
	return []FileChange{change, sc}, nil
}

// ClaudeConfigPreview returns what `casebook setup claude --print` shows: a
// header naming the target file, then the {"mcpServers": {"casebook": ...}}
// document with two-space indentation and a trailing newline. With
// opts.Hooks it appends a blank line, a header naming settings.json and the
// {"hooks": ...} document Casebook merges there. It reads and
// writes nothing, so the desktop panel can show the same text (D-09).
func ClaudeConfigPreview(cliPath string, opts ClaudeOptions) string {
	servers := &jsonObject{}
	servers.Set(claudeServerName, ClaudeMCPServer(cliPath, opts))
	srv, err := servers.Encode()
	if err != nil {
		panic("integrations: encode Claude preview: " + err.Error())
	}
	doc := &jsonObject{}
	doc.Set(claudeServersKey, bytes.TrimRight(srv, "\n"))
	out, err := doc.Encode()
	if err != nil {
		panic("integrations: encode Claude preview: " + err.Error())
	}
	text := `Claude Code MCP server (user scope): merge into the top-level "mcpServers" object of ` +
		opts.Paths.ClaudeJSON + "\n" + string(out)
	if opts.Hooks {
		hooks := &jsonObject{}
		hooks.Set(claudeHooksKey, claudeHooksDocument(cliPath, opts))
		h, err := hooks.Encode()
		if err != nil {
			panic("integrations: encode Claude hooks preview: " + err.Error())
		}
		text += "\n" + `Claude Code hooks: merge into the top-level "hooks" object of ` +
			opts.Paths.ClaudeSettings + "\n" + string(h)
	}
	return text
}
