package mcpserver

import (
	"context"
	"encoding/json"
	"testing"
)

// tools/list size bounds (AR-05). The server is registered at user scope,
// so every Claude Code and Codex session pays for this listing in context.
// Before AR-05 it was about 64 KB; raise these only with a reason.
//
// Raised from 40000 to 45000 for the Evernote note tools, then lowered to
// 42000 when they were removed: the listing is about 40.4 KB, a little over
// the old 40000 because casebook_get_report gained its brief mode and the
// rules for handing a case to a note or document app.
const (
	maxToolsListBytes       = 42000
	maxToolInputSchemaBytes = 6000
	maxToolOutputSchemaSize = 64
)

func TestToolsListSize(t *testing.T) {
	a, _ := seeded(t)
	cs := newTestSession(t, a, sessionOpts{})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("tools/list: %d bytes for %d tools", len(b), len(res.Tools))
	if len(b) > maxToolsListBytes {
		t.Errorf("tools/list is %d bytes, want at most %d", len(b), maxToolsListBytes)
	}
	for _, tool := range res.Tools {
		in, _ := json.Marshal(tool.InputSchema)
		out, _ := json.Marshal(tool.OutputSchema)
		if len(in) > maxToolInputSchemaBytes {
			t.Errorf("%s: input schema is %d bytes, want at most %d", tool.Name, len(in), maxToolInputSchemaBytes)
		}
		if len(out) > maxToolOutputSchemaSize {
			t.Errorf("%s: output schema is %d bytes, want at most %d", tool.Name, len(out), maxToolOutputSchemaSize)
		}
	}
}
