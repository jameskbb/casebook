package mcpserver

import "testing"

func TestClientName(t *testing.T) {
	cases := []struct {
		raw, override, want string
	}{
		{"claude-code", "", "claude-code"},
		{"Claude Code", "", "claude-code"},
		{"codex-mcp-client", "", "codex"},
		{"Codex", "", "codex"},
		{" CLAUDE-CODE ", "", "claude-code"},
		{"my-claude-desktop-thing", "", "my-claude-desktop-thing"},
		{"codex-fork", "", "codex-fork"},
		{"cursor", "", "cursor"},
		{"", "", ""},
		{"claude-code", "my-agent", "my-agent"},
		{"claude-code", "   ", "claude-code"},
	}
	for _, c := range cases {
		if got := ClientName(c.raw, c.override); got != c.want {
			t.Errorf("ClientName(%q, %q) = %q, want %q", c.raw, c.override, got, c.want)
		}
	}
}

func TestClientInfoNameNil(t *testing.T) {
	if got := clientInfoName(nil); got != "" {
		t.Errorf("clientInfoName(nil) = %q, want empty", got)
	}
}
