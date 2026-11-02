package integrations

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// The Casebook-managed block in Codex's config.toml (D-08). Markers must be
// whole lines; a trailing carriage return is ignored when matching.
const (
	codexBlockBegin = `# BEGIN casebook (managed by "casebook setup codex"; remove with "casebook setup codex --remove")`
	codexBlockEnd   = `# END casebook`
	// codexBlockNote is the comment line after BEGIN. The block ends inside
	// the [mcp_servers.casebook] table, so a key appended below it would
	// belong to that table, not to the root (IN-11).
	codexBlockNote = `# Keep root-level keys above this block: TOML reads keys below a table header as part of that table.`
)

// Codex config keys (D-02).
const (
	codexServersKey  = "mcp_servers"
	codexServerName  = "casebook"
	codexServerEntry = codexServersKey + "." + codexServerName
	codexTableHeader = "[" + codexServerEntry + "]"
)

// Refusal reasons for Codex config. None of them quotes file contents.
const (
	unmanagedCodexReason = `an existing [mcp_servers.casebook] is not managed by Casebook; edit it by hand or remove it, then rerun`
	damagedMarkersReason = `the Casebook block markers in this file are damaged; fix or remove them by hand`
	inseparableReason    = `the rest of the file does not parse without the Casebook block; fix it by hand`
	unmanagedRemainsNote = `a hand-written [mcp_servers.casebook] remains; remove it by hand if you no longer want it`
	orphanRemainsNote    = `[mcp_servers.casebook] keys written outside the Casebook block remain without a command ` +
		`(such as a [mcp_servers.casebook.env] table); Codex may refuse to load its config until you remove them by hand`
	blockExtrasReason = `the Casebook block holds your own additions that Casebook cannot carry into the updated block safely; ` +
		`move them out of the block or remove the block by hand, then rerun`
)

// CodexOptions configures the Codex setup functions.
type CodexOptions struct {
	// Paths locates the config files (from ResolvePaths).
	Paths Paths
	// DBPath, when set, is an absolute database path; the server's args
	// become ["mcp", "--db", DBPath].
	DBPath string
	// Now is the backup timestamp clock; nil means time.Now.
	Now func() time.Time
}

// now returns the current time from opts.Now or time.Now.
func (o CodexOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// codexServer is the [mcp_servers.casebook] table Casebook writes. Its
// values are rendered by toml.Marshal, so any path is quoted correctly.
type codexServer struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
}

// blockSpan locates the managed block: [Start, End) covers the BEGIN line
// through the END line and its line ending (or to the end of the data when
// the END line has none).
type blockSpan struct {
	Start, End int
	Found      bool
}

// errDamagedMarkers is returned for a BEGIN without an END, an END without
// a BEGIN, or a second BEGIN.
var errDamagedMarkers = errors.New(damagedMarkersReason)

// findManagedBlock finds the one managed block in data. Data without any
// marker line reports Found false.
func findManagedBlock(data []byte) (blockSpan, error) {
	var span blockSpan
	open := false
	for off := 0; off < len(data); {
		line, next := data[off:], len(data)
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line, next = line[:i], off+i+1
		}
		switch string(bytes.TrimSuffix(line, []byte("\r"))) {
		case codexBlockBegin:
			if open || span.Found {
				return blockSpan{}, errDamagedMarkers
			}
			open, span.Start = true, off
		case codexBlockEnd:
			if !open {
				return blockSpan{}, errDamagedMarkers
			}
			open, span.End, span.Found = false, next, true
		}
		off = next
	}
	if open {
		return blockSpan{}, errDamagedMarkers
	}
	return span, nil
}

// cutSpan returns a copy of data without [start, end).
func cutSpan(data []byte, start, end int) []byte {
	out := make([]byte, 0, len(data)-(end-start))
	out = append(out, data[:start]...)
	return append(out, data[end:]...)
}

// tomlError is a TOML parse failure reduced to its position: the decoder's
// own message quotes document lines, which may hold secrets.
type tomlError struct{ row, col int }

func (e *tomlError) Error() string {
	if e.row == 0 {
		return "not valid TOML"
	}
	return fmt.Sprintf("not valid TOML (line %d, column %d)", e.row, e.col)
}

// codexCasebook parses data as TOML and returns its mcp_servers.casebook
// value. found is true when that key exists, even when it is not a table
// (server is then nil). An mcp_servers value that is not a table means not
// found. Errors are *tomlError and never contain document text.
func codexCasebook(data []byte) (server map[string]any, found bool, err error) {
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		var de *toml.DecodeError
		if errors.As(err, &de) {
			row, col := de.Position()
			return nil, false, &tomlError{row: row, col: col}
		}
		return nil, false, &tomlError{}
	}
	servers, ok := doc[codexServersKey].(map[string]any)
	if !ok {
		return nil, false, nil
	}
	v, ok := servers[codexServerName]
	if !ok {
		return nil, false, nil
	}
	server, _ = v.(map[string]any)
	return server, true, nil
}

// codexBlock renders the managed block with every line ending in eol: the
// BEGIN line, the note that root-level keys belong above the block, the
// [mcp_servers.casebook] header, the toml.Marshal output of
// command and args (D-03), and the END line. It never contains a bare
// [mcp_servers] header, which would redefine a table the file may declare.
func codexBlock(cliPath string, opts CodexOptions, eol string) string {
	return codexBlockWith(cliPath, opts, eol, nil, nil)
}

// codexBlockWith is codexBlock with the user's own block lines carried over
// (WR-04): pre goes between the BEGIN line and the table header, post
// between the command and args and the END line.
func codexBlockWith(cliPath string, opts CodexOptions, eol string, pre, post []string) string {
	body, err := toml.Marshal(codexServer{Command: cliPath, Args: mcpArgs(opts.DBPath)})
	if err != nil {
		// A string and a string slice always encode.
		panic("integrations: encode Codex MCP server: " + err.Error())
	}
	lines := []string{codexBlockBegin, codexBlockNote}
	lines = append(lines, pre...)
	lines = append(lines, codexTableHeader)
	lines = append(lines, strings.Split(strings.TrimRight(string(body), "\n"), "\n")...)
	lines = append(lines, post...)
	lines = append(lines, codexBlockEnd)
	return strings.Join(lines, eol) + eol
}

// codexManagedKey reports whether a block line inside the
// [mcp_servers.casebook] table sets command or args, the keys setup owns.
func codexManagedKey(trimmed string) bool {
	for _, k := range []string{"command", "args"} {
		if rest, ok := strings.CutPrefix(trimmed, k); ok && strings.HasPrefix(strings.TrimLeft(rest, " \t"), "=") {
			return true
		}
	}
	return false
}

// blockExtras returns the lines of an existing managed block that setup
// carries into the new block (WR-04), without line endings: every line but
// the BEGIN and END markers, the note line after BEGIN, the
// [mcp_servers.casebook] header and the
// command and args lines of that table. pre are the lines before the
// header, post the lines after it, in order. A value that spans lines (a
// multi-line args array) leaves fragments behind; the merged file then
// fails the read-back check and setup refuses instead of guessing.
func blockExtras(block []byte) (pre, post []string) {
	text := strings.ReplaceAll(string(block), "\r\n", "\n")
	inTable, seenHeader := false, false
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		switch {
		case line == codexBlockBegin, line == codexBlockEnd, line == codexBlockNote:
			continue
		case trimmed == codexTableHeader && !seenHeader:
			inTable, seenHeader = true, true
			continue
		case strings.HasPrefix(trimmed, "["):
			inTable = false
		case inTable && codexManagedKey(trimmed):
			continue
		}
		if seenHeader {
			post = append(post, line)
		} else {
			pre = append(pre, line)
		}
	}
	return pre, post
}

// codexDocWithoutManaged parses data and drops mcp_servers.casebook's
// command and args, for comparing what a re-rendered block kept.
func codexDocWithoutManaged(data []byte) (map[string]any, error) {
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if servers, ok := doc[codexServersKey].(map[string]any); ok {
		if server, ok := servers[codexServerName].(map[string]any); ok {
			delete(server, "command")
			delete(server, "args")
		}
	}
	return doc, nil
}

// CodexConfigPreview returns what `casebook setup codex --print` shows: a
// comment naming the target file, then the managed block with newline line
// endings. The whole text is valid TOML. It reads and writes nothing, so the
// desktop panel can show the same text (D-09).
func CodexConfigPreview(cliPath string, opts CodexOptions) string {
	return "# Codex MCP server: add this block to " + opts.Paths.CodexConfig + "\n" +
		codexBlock(cliPath, opts, "\n")
}

// fileEOL is the line ending of data: CRLF when it holds any, else LF.
func fileEOL(data []byte) string {
	if bytes.Contains(data, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// checkCodexServer reports why a parsed mcp_servers.casebook differs from
// the intended command and args, or "" when it matches. Besides command and
// args it may hold only the keys in blockKeys: the keys the user added
// inside the Casebook block (WR-04). Any other key comes from outside the
// block, such as a root key appended after a block at the end of the file.
func checkCodexServer(server map[string]any, found bool, cliPath string, args []string, blockKeys []string) string {
	if !found || server == nil {
		return codexServerEntry + " would be missing"
	}
	keys := make([]string, 0, len(server))
	for k := range server {
		if k != "command" && k != "args" && !slices.Contains(blockKeys, k) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		return codexServerEntry + " would hold keys from outside the Casebook block"
	}
	if cmd, ok := server["command"].(string); !ok || cmd != cliPath {
		return "the command would not read back as written"
	}
	got, ok := server["args"].([]any)
	if !ok || len(got) != len(args) {
		return "the args would not read back as written"
	}
	for i, a := range got {
		if s, ok := a.(string); !ok || s != args[i] {
			return "the args would not read back as written"
		}
	}
	return ""
}

// mergedRefusal is the reason for new bytes that would not load as intended.
func mergedRefusal(short string) string {
	return "the merged file would not load as intended (" + short + ")"
}

// codexDoc is a Codex config.toml as read and checked for a change.
type codexDoc struct {
	file configFile
	span blockSpan
	// unmanaged is true when mcp_servers.casebook exists outside the block.
	unmanaged bool
}

// readCodexDoc reads config.toml and refuses a file Casebook should not
// change: invalid TOML, damaged block markers, or a file that does not parse
// once the block is taken out.
func readCodexDoc(path string) (codexDoc, error) {
	f, err := readConfigFile(path)
	if err != nil {
		return codexDoc{}, err
	}
	refuse := func(reason string) (codexDoc, error) {
		return codexDoc{}, &RefusedError{File: f.Path, Reason: reason}
	}
	if _, _, err := codexCasebook(f.Data); err != nil {
		return refuse(err.Error())
	}
	span, err := findManagedBlock(f.Data)
	if err != nil {
		return refuse(err.Error())
	}
	outside := f.Data
	if span.Found {
		outside = cutSpan(f.Data, span.Start, span.End)
	}
	_, unmanaged, err := codexCasebook(outside)
	if err != nil {
		return refuse(inseparableReason)
	}
	return codexDoc{file: f, span: span, unmanaged: unmanaged}, nil
}

// SetupCodex registers the Casebook MCP server in Codex's config.toml
// (opts.Paths.CodexConfig) inside the managed block, leaving every byte
// outside the block as it is (D-08). An existing block is replaced in place,
// carrying over any lines the user added inside it (such as
// startup_timeout_sec or a [mcp_servers.casebook.env] table; WR-04);
// otherwise the block is appended after one blank line, using the file's
// line ending. A [mcp_servers.casebook] outside the block is never touched:
// the run is refused. The new text is parsed again before writing and must
// hold exactly the intended command and args and, for an existing block,
// every other value the file held before; when it does not, the run is
// refused rather than dropping the user's additions. A run that changes
// nothing writes nothing and reports unchanged.
func SetupCodex(cliPath string, opts CodexOptions) (FileChange, error) {
	d, err := readCodexDoc(opts.Paths.CodexConfig)
	if err != nil {
		return FileChange{}, err
	}
	f := d.file
	refuse := func(reason string) (FileChange, error) {
		return FileChange{}, &RefusedError{File: f.Path, Reason: reason}
	}
	if d.unmanaged {
		return refuse(unmanagedCodexReason)
	}
	eol := fileEOL(f.Data)
	block := codexBlock(cliPath, opts, eol)
	var (
		data      []byte
		blockKeys []string
	)
	switch {
	case d.span.Found:
		// Tables Codex appended inside the block move out, right after
		// it (codexforeign.go).
		own, foreign := splitForeign(f.Data[d.span.Start:d.span.End])
		// The keys the user added inside the block may stay.
		blockServer, _, err := codexCasebook(own)
		if err != nil {
			return refuse(blockExtrasReason)
		}
		for k := range blockServer {
			blockKeys = append(blockKeys, k)
		}
		pre, post := blockExtras(own)
		if len(foreign) > 0 {
			// The blank line that separated the moved table stays out.
			for len(post) > 0 && strings.TrimSpace(post[len(post)-1]) == "" {
				post = post[:len(post)-1]
			}
		}
		data = append(data, f.Data[:d.span.Start]...)
		data = append(data, codexBlockWith(cliPath, opts, eol, pre, post)...)
		if len(foreign) > 0 {
			data = append(data, eol...)
			data = append(data, withEOL(foreign, eol)...)
		}
		data = append(data, f.Data[d.span.End:]...)
	case len(f.Data) == 0:
		data = []byte(block)
	default:
		data = append(data, f.Data...)
		if !bytes.HasSuffix(data, []byte("\n")) {
			data = append(data, eol...)
		}
		data = append(data, eol...)
		data = append(data, block...)
	}

	args := mcpArgs(opts.DBPath)
	server, found, err := codexCasebook(data)
	if err != nil {
		if d.span.Found {
			// The carried-over lines did not survive the re-render.
			return refuse(blockExtrasReason)
		}
		var te *tomlError
		if errors.As(err, &te) && te.row > 0 {
			return refuse(mergedRefusal(fmt.Sprintf("it would not be valid TOML at line %d, column %d", te.row, te.col)))
		}
		return refuse(mergedRefusal("it would not be valid TOML"))
	}
	if why := checkCodexServer(server, found, cliPath, args, blockKeys); why != "" {
		return refuse(mergedRefusal(why))
	}
	if d.span.Found {
		// Everything but command and args must read back as before.
		before, berr := codexDocWithoutManaged(f.Data)
		after, aerr := codexDocWithoutManaged(data)
		if berr != nil || aerr != nil || !reflect.DeepEqual(before, after) {
			return refuse(blockExtrasReason)
		}
	}
	if span, err := findManagedBlock(data); err != nil || !span.Found {
		return refuse(mergedRefusal("the block markers would not read back"))
	}

	change := FileChange{File: f.Path, Entries: []string{codexServerEntry}}
	backup, changed, err := f.commit(data, opts.now())
	if err != nil {
		return FileChange{}, err
	}
	switch {
	case !changed:
		change.Action = ActionUnchanged
	case d.span.Found:
		change.Action = ActionUpdated
	default:
		change.Action = ActionAdded
	}
	change.Backup = backup
	return change, nil
}

// blankLineBefore reports the length of the empty line (LF or CRLF) that
// ends data[:start], or 0 when the line just before start is not empty.
func blankLineBefore(data []byte, start int) int {
	head := data[:start]
	for _, eol := range []string{"\r\n", "\n"} {
		// The blank line is eol preceded by the previous line's ending (or
		// nothing, when it is the first line of the file).
		if bytes.HasSuffix(head, []byte(eol)) {
			rest := head[:len(head)-len(eol)]
			if len(rest) == 0 || bytes.HasSuffix(rest, []byte("\n")) {
				return len(eol)
			}
		}
	}
	return 0
}

// blankLineAfter reports the length of the empty line (LF or CRLF) that
// starts data[end:], or 0 when there is none.
func blankLineAfter(data []byte, end int) int {
	tail := data[end:]
	switch {
	case bytes.HasPrefix(tail, []byte("\r\n")):
		return 2
	case bytes.HasPrefix(tail, []byte("\n")):
		return 1
	}
	return 0
}

// remainsNote is the remove note for a file whose remaining text (the file
// without the block) still defines mcp_servers.casebook: hand-written keys
// outside the block are never touched, and when they hold no command (an
// orphaned [mcp_servers.casebook.env], say) the note warns that Codex may
// refuse the config.
func remainsNote(rest []byte) string {
	server, found, err := codexCasebook(rest)
	if err != nil || !found {
		return ""
	}
	if _, ok := server["command"]; !ok {
		return orphanRemainsNote
	}
	return unmanagedRemainsNote
}

// RemoveCodex deletes the Casebook-managed block from Codex's config.toml,
// plus one adjacent blank line (the separator setup added before it when
// present, else one right after it), and nothing else (D-08, INT-06).
// Everything of Casebook's inside the block goes with it, including keys or
// tables such as [mcp_servers.casebook.env] the user added there; tables
// that are not Casebook's (such as a [projects."<path>"] trust entry Codex
// appended inside the block) stay where the block was; nothing outside the
// block is ever removed. A missing file is never created; a file without the block
// reports unchanged and is not written. A note says when a hand-written
// [mcp_servers.casebook] remains, and warns when what remains has no
// command. Invalid TOML and damaged markers refuse.
func RemoveCodex(opts CodexOptions) (FileChange, error) {
	d, err := readCodexDoc(opts.Paths.CodexConfig)
	if err != nil {
		return FileChange{}, err
	}
	f := d.file
	change := FileChange{File: f.Path, Action: ActionUnchanged, Entries: []string{codexServerEntry}}
	if !d.span.Found {
		if d.unmanaged {
			change.Note = remainsNote(f.Data)
		}
		return change, nil
	}
	start, end := d.span.Start, d.span.End
	_, foreign := splitForeign(f.Data[start:end])
	var data []byte
	if len(foreign) > 0 {
		// Tables Codex appended inside the block stay, in its place.
		data = append(data, f.Data[:start]...)
		data = append(data, withEOL(bytes.TrimLeft(foreign, "\r\n"), fileEOL(f.Data))...)
		data = append(data, f.Data[end:]...)
	} else {
		if n := blankLineBefore(f.Data, start); n > 0 {
			start -= n
		} else {
			end += blankLineAfter(f.Data, end)
		}
		data = cutSpan(f.Data, start, end)
	}
	if _, _, err := codexCasebook(data); err != nil {
		return FileChange{}, &RefusedError{File: f.Path, Reason: "the file would not be valid TOML without the Casebook block; fix it by hand"}
	}
	if d.unmanaged {
		change.Note = remainsNote(data)
	}
	if span, err := findManagedBlock(data); err != nil || span.Found {
		return FileChange{}, &RefusedError{File: f.Path, Reason: damagedMarkersReason}
	}
	backup, changed, err := f.commit(data, opts.now())
	if err != nil {
		return FileChange{}, err
	}
	if changed {
		change.Action = ActionRemoved
		change.Backup = backup
	}
	return change, nil
}
