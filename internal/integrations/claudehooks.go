package integrations

import (
	"bytes"
	"encoding/json"
	"errors"
	"runtime"
	"strings"
)

// Claude Code settings.json keys for hooks (D-02, D-07).
const claudeHooksKey = "hooks"

// hookSpec is one Casebook hook: the Claude Code event, the matcher of its
// group ("" for events without matchers) and the casebook hook subcommand.
type hookSpec struct{ Event, Matcher, Sub string }

// casebookHookSpecs are the hooks `setup claude --hooks` installs, in the
// order they are written (D-07).
var casebookHookSpecs = []hookSpec{
	{Event: "SessionStart", Matcher: "startup|resume|clear|compact", Sub: "session-start"},
	{Event: "Stop", Sub: "checkpoint"},
	{Event: "PreCompact", Matcher: "manual|auto", Sub: "checkpoint"},
}

// hookTimeoutSeconds is the timeout of every Casebook hook handler (Claude
// Code reads it in seconds).
const hookTimeoutSeconds = 10

// casebookHookSubs are the casebook hook subcommands a Casebook handler can
// run.
var casebookHookSubs = []string{"session-start", "checkpoint"}

// hookHandler is one command handler inside a hook group, with fields in the
// order Claude Code documents them. Args is set only for an exec-form
// handler (see hookInvocation).
type hookHandler struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Timeout int      `json:"timeout"`
}

// hookGroup is one matcher group of a hook event.
type hookGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookHandler `json:"hooks"`
}

// hookEntries are the Casebook-owned settings keys a hooks run touches.
func hookEntries() []string {
	out := make([]string, 0, len(casebookHookSpecs))
	for _, s := range casebookHookSpecs {
		out = append(out, claudeHooksKey+"."+s.Event)
	}
	return out
}

// QuotePOSIX quotes s as one word for a POSIX shell (sh, bash). s is
// returned unchanged when it is not empty and ShellSafe; otherwise it is
// wrapped in single quotes, each embedded quote closing the quoted text,
// escaped with a backslash, then reopening it. Inside single quotes the
// shell expands nothing, so $, backticks, % and backslashes stay literal.
// D-07 asks for quotes when the path has a space; quoting any other
// shell-special byte too keeps paths with $, &, quotes or globs safe (a
// discretion choice that extends D-07).
func QuotePOSIX(s string) string {
	if s != "" && ShellSafe(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ShellSafe reports whether every byte of s is a letter, a digit or one of
// _ . / + , : @ % = -, so s needs no quoting as a POSIX shell word.
func ShellSafe(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.IndexByte("_./+,:@%=-", c) >= 0:
		default:
			return false
		}
	}
	return true
}

// Flags a hook command may end with after "hook <sub>". QuietFlag silences
// the session-start hint when no investigation is active (setup claude
// --hooks --quiet-when-inactive); hintFlag is the opt-in spelling hook
// commands carried while the hint was off by default, still recognized so a
// rerun of setup replaces those commands instead of adding a second one.
const (
	QuietFlag = "--quiet-when-inactive"
	hintFlag  = "--hint-when-inactive"
)

// hookArgs are the arguments of a Casebook hook handler after the program:
// [--db <db>] hook <sub> [--quiet-when-inactive] [--scope marked]. The
// global --db flag goes before the subcommand; the trailing flags, written
// only when asked for (setup claude --hooks --quiet-when-inactive, --scope
// marked), go last, so the arguments always end with "hook <sub>" once they
// are stripped (casebookArgsSub, D-06). Quiet applies to session-start only.
func hookArgs(dbPath, sub, scope string, quiet bool) []string {
	var args []string
	if dbPath != "" {
		args = append(args, "--db", dbPath)
	}
	args = append(args, "hook", sub)
	if quiet && sub == "session-start" {
		args = append(args, QuietFlag)
	}
	if scope == ScopeMarked {
		args = append(args, "--scope", ScopeMarked)
	}
	return args
}

// hookInvocation is the command and args of a Casebook hook handler.
//
// On macOS and Linux Claude Code runs a shell-form handler (no args) with
// sh, so the handler is the shell form: the command string is the CLI path
// and hookArgs, each quoted with QuotePOSIX, and args is nil.
//
// On Windows Claude Code runs a shell-form handler through Git Bash, or
// through PowerShell when Git for Windows is not installed, and no quoting
// suits both: PowerShell rejects a command line that starts with a quoted
// string, and an unquoted Windows path breaks on spaces. So on Windows the
// handler is the exec form (Claude Code 2.1.139 and later): command is the
// CLI path as is and args are hookArgs, which Claude Code spawns directly
// with no shell, so no character in either path needs quoting.
func hookInvocation(cliPath, dbPath, sub, scope string, quiet bool, goos string) (command string, args []string) {
	args = hookArgs(dbPath, sub, scope, quiet)
	if goos == "windows" {
		return cliPath, args
	}
	var b strings.Builder
	b.WriteString(QuotePOSIX(cliPath))
	for _, a := range args {
		b.WriteByte(' ')
		b.WriteString(QuotePOSIX(a))
	}
	return b.String(), nil
}

// trimHookFlags strips the trailing hook flags setup writes or wrote
// ("--scope all", "--scope marked", the "--scope=" spellings,
// --quiet-when-inactive and --hint-when-inactive), in any order, from args.
func trimHookFlags(args []string) []string {
	for {
		n := len(args)
		switch {
		case n >= 2 && args[n-2] == "--scope" && validScopeValue(args[n-1]):
			args = args[:n-2]
		case n >= 1 && strings.HasPrefix(args[n-1], "--scope=") && validScopeValue(strings.TrimPrefix(args[n-1], "--scope=")):
			args = args[:n-1]
		case n >= 1 && (args[n-1] == QuietFlag || args[n-1] == hintFlag):
			args = args[:n-1]
		default:
			return args
		}
	}
}

// hookFlagSuffixes are the shell-form spellings of the flags trimHookFlags
// strips, each with the space that separates it from the word before.
var hookFlagSuffixes = []string{
	" --scope " + ScopeAll, " --scope " + ScopeMarked,
	" --scope=" + ScopeAll, " --scope=" + ScopeMarked,
	" " + QuietFlag, " " + hintFlag,
}

// trimHookFlagSuffixes is trimHookFlags for a shell-form command string.
func trimHookFlagSuffixes(cmd string) string {
	for {
		trimmed := false
		for _, suffix := range hookFlagSuffixes {
			if rest, found := strings.CutSuffix(cmd, suffix); found {
				cmd, trimmed = rest, true
				break
			}
		}
		if !trimmed {
			return cmd
		}
	}
}

// validScopeValue reports whether v is a scope name as written in a hook
// command.
func validScopeValue(v string) bool { return v == ScopeAll || v == ScopeMarked }

// casebookArgsSub reports which hook subcommand an argument list ends with:
// "hook <sub>", optionally followed by the flags trimHookFlags strips.
func casebookArgsSub(args []string) (string, bool) {
	args = trimHookFlags(args)
	n := len(args)
	if n < 2 || args[n-2] != "hook" {
		return "", false
	}
	for _, s := range casebookHookSubs {
		if args[n-1] == s {
			return s, true
		}
	}
	return "", false
}

// casebookHookSub reports whether handler is a Casebook hook handler and, if
// so, which hook subcommand it runs (D-06). A handler is Casebook's when it
// is an object whose command member is a JSON string that contains
// "casebook" and either
//
//   - it has an args member (the exec form setup writes on Windows) that is
//     an array of strings ending with "hook session-start" or "hook
//     checkpoint", optionally followed by "--scope <all|marked>",
//     --quiet-when-inactive or --hint-when-inactive; or
//   - it has no args member (the shell form) and the command ends with
//     " hook session-start" or " hook checkpoint", optionally followed by
//     the same flags. This also matches every shell form earlier setups
//     wrote, with the path unquoted, single-quoted or double-quoted, and
//     the flag a person added by hand following the docs, so a rerun of
//     setup replaces them.
//
// Anything else belongs to the user.
func casebookHookSub(handler json.RawMessage) (sub string, ok bool) {
	var h map[string]json.RawMessage
	if err := json.Unmarshal(handler, &h); err != nil || h == nil {
		return "", false
	}
	raw, present := h["command"]
	if !present {
		return "", false
	}
	var cmd string
	if err := json.Unmarshal(raw, &cmd); err != nil {
		return "", false
	}
	if !strings.Contains(cmd, "casebook") {
		return "", false
	}
	if rawArgs, present := h["args"]; present {
		var args []string
		if !isJSONArray(rawArgs) || json.Unmarshal(rawArgs, &args) != nil {
			return "", false
		}
		return casebookArgsSub(args)
	}
	rest := trimHookFlagSuffixes(cmd)
	for _, s := range casebookHookSubs {
		if strings.HasSuffix(rest, " hook "+s) {
			return s, true
		}
	}
	return "", false
}

// hookGOOS is the command form of opts: opts.GOOS, else runtime.GOOS.
func hookGOOS(opts ClaudeOptions) string {
	if opts.GOOS != "" {
		return opts.GOOS
	}
	return runtime.GOOS
}

// specInvocation is hookInvocation for spec under opts.
func specInvocation(spec hookSpec, cliPath string, opts ClaudeOptions) (string, []string) {
	return hookInvocation(cliPath, opts.DBPath, spec.Sub, opts.HookScope, opts.HookQuiet, hookGOOS(opts))
}

// desiredHookGroup is the one-handler group Casebook installs for spec.
func desiredHookGroup(spec hookSpec, cliPath string, opts ClaudeOptions) json.RawMessage {
	command, args := specInvocation(spec, cliPath, opts)
	raw, err := marshalNoEscape(hookGroup{
		Matcher: spec.Matcher,
		Hooks: []hookHandler{{
			Type:    "command",
			Command: command,
			Args:    args,
			Timeout: hookTimeoutSeconds,
		}},
	})
	if err != nil {
		// Strings and an int always encode.
		panic("integrations: encode hook group: " + err.Error())
	}
	return raw
}

// mergedHookGroup is the Casebook group for spec built from an existing
// Casebook group (one made only of Casebook handlers), keeping what the user
// tuned (WR-04): the group's matcher member, present or absent, and its
// other members stay as they are, and the group's handler for spec.Sub keeps
// its timeout and other members, with type, command and args set to
// Casebook's (args removed for the shell form). A group without such a
// handler gets the desired handler. Any other Casebook handlers in the
// group are dropped, as before. A group that does not parse is replaced by
// the desired group.
func mergedHookGroup(existing json.RawMessage, spec hookSpec, cliPath string, opts ClaudeOptions) json.RawMessage {
	group, handlers, ok := groupHandlers(existing)
	if !ok {
		return desiredHookGroup(spec, cliPath, opts)
	}
	handler := &jsonObject{}
	for _, h := range handlers {
		if sub, ok := casebookHookSub(h); ok && sub == spec.Sub {
			if o, err := parseJSONObject(h); err == nil {
				handler = o
			}
			break
		}
	}
	command, args := specInvocation(spec, cliPath, opts)
	handler.Set("type", mustMarshal("command"))
	handler.Set("command", mustMarshal(command))
	if args != nil {
		handler.Set("args", mustMarshal(args))
	} else {
		handler.Delete("args")
	}
	if _, ok := handler.Get("timeout"); !ok {
		handler.Set("timeout", mustMarshal(hookTimeoutSeconds))
	}
	group.Set(claudeHooksKey, encodeRawArray([]json.RawMessage{encodeCompact(handler)}))
	return encodeCompact(group)
}

// mustMarshal is marshalNoEscape for values that always encode (strings and
// ints).
func mustMarshal(v any) json.RawMessage {
	raw, err := marshalNoEscape(v)
	if err != nil {
		panic("integrations: encode hook value: " + err.Error())
	}
	return raw
}

// encodeCompact renders o as compact JSON, keeping its member order and
// values.
func encodeCompact(o *jsonObject) json.RawMessage {
	enc, err := o.Encode()
	if err != nil {
		// Members parsed from JSON or set from mustMarshal always encode.
		panic("integrations: encode hook group: " + err.Error())
	}
	var b bytes.Buffer
	if err := json.Compact(&b, enc); err != nil {
		panic("integrations: compact hook group: " + err.Error())
	}
	return json.RawMessage(b.Bytes())
}

// claudeHooksDocument is the hooks object Casebook installs, as compact
// JSON: {"SessionStart": [group], "Stop": [group], "PreCompact": [group]}
// in casebookHookSpecs order.
func claudeHooksDocument(cliPath string, opts ClaudeOptions) json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, spec := range casebookHookSpecs {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := marshalNoEscape(spec.Event)
		if err != nil {
			panic("integrations: encode hook event: " + err.Error())
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(encodeRawArray([]json.RawMessage{desiredHookGroup(spec, cliPath, opts)}))
	}
	b.WriteByte('}')
	return json.RawMessage(b.Bytes())
}

// encodeRawArray joins raw JSON values into a compact array, keeping each
// element's bytes.
func encodeRawArray(items []json.RawMessage) json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('[')
	for i, it := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(it)
	}
	b.WriteByte(']')
	return json.RawMessage(b.Bytes())
}

// isJSONArray reports whether the raw value is an array.
func isJSONArray(v json.RawMessage) bool {
	t := bytes.TrimLeft(v, jsonSpace)
	return len(t) > 0 && t[0] == '['
}

// decodeRawArray splits an array value into its raw elements. errNotArray
// when v is not an array.
func decodeRawArray(v json.RawMessage) ([]json.RawMessage, error) {
	if !isJSONArray(v) {
		return nil, errNotArray
	}
	var items []json.RawMessage
	if err := json.Unmarshal(v, &items); err != nil {
		return nil, errNotArray
	}
	return items, nil
}

// errNotArray marks a value that is not a JSON array.
var errNotArray = errors.New("not an array")

// groupHandlers returns the raw handlers of a hook group: ok is false when
// group is not an object with a "hooks" array.
func groupHandlers(group json.RawMessage) (obj *jsonObject, handlers []json.RawMessage, ok bool) {
	if !isJSONObject(group) {
		return nil, nil, false
	}
	o, err := parseJSONObject(group)
	if err != nil {
		return nil, nil, false
	}
	raw, present := o.Get(claudeHooksKey)
	if !present {
		return nil, nil, false
	}
	hs, err := decodeRawArray(raw)
	if err != nil {
		return nil, nil, false
	}
	return o, hs, true
}

// groupKind classifies one element of a hook event array.
type groupKind int

const (
	// groupUser is anything that is not entirely Casebook's.
	groupUser groupKind = iota
	// groupCasebook is a group whose non-empty hooks list holds only
	// Casebook handlers.
	groupCasebook
)

// classifyGroup reports whether group is a Casebook group, and whether it
// holds a Casebook handler running sub (a user group holding one is
// "mixed").
func classifyGroup(group json.RawMessage, sub string) (kind groupKind, hasSub bool) {
	_, hs, ok := groupHandlers(group)
	if !ok || len(hs) == 0 {
		return groupUser, false
	}
	all := true
	for _, h := range hs {
		s, isCasebook := casebookHookSub(h)
		if !isCasebook {
			all = false
			continue
		}
		if s == sub {
			hasSub = true
		}
	}
	if all {
		return groupCasebook, hasSub
	}
	return groupUser, hasSub
}

// hooksObject parses settings' hooks member. A missing member is an empty
// object; anything but an object is refused.
func hooksObject(settings *jsonObject) (*jsonObject, error) {
	hooks, present, err := settings.childObject(claudeHooksKey)
	if err != nil {
		return nil, err
	}
	if !present {
		return &jsonObject{}, nil
	}
	return hooks, nil
}

// hasCasebookHandler reports whether any Casebook event of hooks holds a
// Casebook handler. Malformed values count as none.
func hasCasebookHandler(hooks *jsonObject) bool {
	for _, spec := range casebookHookSpecs {
		raw, ok := hooks.Get(spec.Event)
		if !ok {
			continue
		}
		items, err := decodeRawArray(raw)
		if err != nil {
			continue
		}
		for _, it := range items {
			_, hs, ok := groupHandlers(it)
			if !ok {
				continue
			}
			for _, h := range hs {
				if _, ok := casebookHookSub(h); ok {
					return true
				}
			}
		}
	}
	return false
}

// installHooks merges the Casebook hook groups into settings (D-06). For
// each Casebook event: an existing Casebook group (one made only of Casebook
// handlers) is updated in place (mergedHookGroup keeps the user's matcher
// and timeout), and any further Casebook groups are deleted; with none, a user group that already runs the
// same casebook hook is left alone with a note; otherwise the desired group
// is appended. User groups, including odd shapes, keep their bytes. hooks
// that is not an object, or a Casebook event that is not an array, is
// refused (the error text is the refusal reason). changed is false when the
// document needs no edit.
func installHooks(settings *jsonObject, cliPath string, opts ClaudeOptions) (note string, changed bool, err error) {
	hooks, err := hooksObject(settings)
	if err != nil {
		return "", false, err
	}
	var notes []string
	for _, spec := range casebookHookSpecs {
		var items []json.RawMessage
		if raw, ok := hooks.Get(spec.Event); ok {
			items, err = decodeRawArray(raw)
			if err != nil {
				return "", false, errors.New(claudeHooksKey + "." + spec.Event + " is not an array")
			}
		}
		desired := desiredHookGroup(spec, cliPath, opts)
		var casebookIdx []int
		mixed := false
		for i, it := range items {
			kind, hasSub := classifyGroup(it, spec.Sub)
			switch {
			case kind == groupCasebook:
				casebookIdx = append(casebookIdx, i)
			case hasSub:
				mixed = true
			}
		}
		eventChanged := false
		switch {
		case len(casebookIdx) > 0:
			first := casebookIdx[0]
			if merged := mergedHookGroup(items[first], spec, cliPath, opts); !sameJSON(items[first], merged) {
				items[first] = merged
				eventChanged = true
			}
			if len(casebookIdx) > 1 {
				drop := make(map[int]bool, len(casebookIdx)-1)
				for _, i := range casebookIdx[1:] {
					drop[i] = true
				}
				kept := items[:0:0]
				for i, it := range items {
					if !drop[i] {
						kept = append(kept, it)
					}
				}
				items = kept
				eventChanged = true
			}
		case mixed:
			notes = append(notes, claudeHooksKey+"."+spec.Event+
				" already runs casebook inside a group with your own hooks; left unchanged")
		default:
			items = append(items, desired)
			eventChanged = true
		}
		if eventChanged {
			hooks.Set(spec.Event, encodeRawArray(items))
			changed = true
		}
	}
	if changed {
		enc, err := hooks.Encode()
		if err != nil {
			return "", false, err
		}
		settings.Set(claudeHooksKey, bytes.TrimRight(enc, "\n"))
	}
	return strings.Join(notes, "; "), changed, nil
}

// isCasebookEvent reports whether event is one Casebook installs hooks for.
func isCasebookEvent(event string) bool {
	for _, s := range casebookHookSpecs {
		if s.Event == event {
			return true
		}
	}
	return false
}

// removeHooks deletes every Casebook hook handler from settings (D-06,
// INT-03). A group left with no handlers is dropped; a user group keeps its
// other handlers and members (such as matcher) in order.
//
// Containers are never deleted, the same rule as for mcpServers in
// .claude.json: Casebook cannot tell whether the user created the hooks
// object or an event array, so an event array the removal empties stays as
// [] and the hooks object stays even when it holds only empty arrays. After
// an install that created them, that leaves empty arrays behind; Claude Code
// treats an empty array like a missing event.
//
// hooks that is not an object, or a Casebook event that is not an array, is
// refused; another event that is not an array is skipped. events lists the
// hook events that changed, as "hooks.<Event>", Casebook's events first in
// install order.
func removeHooks(settings *jsonObject) (events []string, err error) {
	hooks, present, err := settings.childObject(claudeHooksKey)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	for _, m := range append([]jsonMember(nil), hooks.Members...) {
		items, err := decodeRawArray(m.Value)
		if err != nil {
			if isCasebookEvent(m.Key) {
				return nil, errors.New(claudeHooksKey + "." + m.Key + " is not an array")
			}
			continue
		}
		kept, changed, err := removeCasebookHandlers(items)
		if err != nil {
			return nil, err
		}
		if !changed {
			continue
		}
		events = append(events, claudeHooksKey+"."+m.Key)
		hooks.Set(m.Key, encodeRawArray(kept))
	}
	if len(events) == 0 {
		return nil, nil
	}
	events = sortHookEvents(events)
	enc, err := hooks.Encode()
	if err != nil {
		return nil, err
	}
	settings.Set(claudeHooksKey, bytes.TrimRight(enc, "\n"))
	return events, nil
}

// sortHookEvents orders "hooks.<Event>" names: Casebook's events in
// casebookHookSpecs order first, then the rest in document order.
func sortHookEvents(events []string) []string {
	out := make([]string, 0, len(events))
	seen := make(map[string]bool, len(events))
	for _, e := range hookEntries() {
		for _, got := range events {
			if got == e {
				out = append(out, e)
				seen[e] = true
			}
		}
	}
	for _, e := range events {
		if !seen[e] {
			out = append(out, e)
		}
	}
	return out
}

// removeCasebookHandlers drops Casebook handlers from the groups of one
// event array. changed reports whether any handler was dropped.
func removeCasebookHandlers(items []json.RawMessage) (kept []json.RawMessage, changed bool, err error) {
	kept = make([]json.RawMessage, 0, len(items))
	for _, it := range items {
		obj, hs, ok := groupHandlers(it)
		if !ok {
			kept = append(kept, it)
			continue
		}
		var left []json.RawMessage
		for _, h := range hs {
			if _, isCasebook := casebookHookSub(h); !isCasebook {
				left = append(left, h)
			}
		}
		if len(left) == len(hs) {
			kept = append(kept, it)
			continue
		}
		changed = true
		if len(left) == 0 {
			continue
		}
		obj.Set(claudeHooksKey, encodeRawArray(left))
		enc, err := obj.Encode()
		if err != nil {
			return nil, false, err
		}
		kept = append(kept, json.RawMessage(bytes.TrimRight(enc, "\n")))
	}
	return kept, changed, nil
}
