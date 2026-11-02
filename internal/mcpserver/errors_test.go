package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
)

func toolErrorText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if !res.IsError {
		t.Fatal("toolError result has IsError false")
	}
	if res.GetError() == nil {
		t.Fatal("toolError result carries no error")
	}
	if len(res.Content) != 1 {
		t.Fatalf("toolError content has %d items, want 1", len(res.Content))
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("toolError content is %T, want text", res.Content[0])
	}
	if tc.Text != res.GetError().Error() {
		t.Fatalf("content %q differs from error %q", tc.Text, res.GetError())
	}
	return tc.Text
}

func TestToolError(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		exact string
		has   []string
	}{
		{
			name:  "validation",
			err:   app.NewValidationError("source_type", "must be one of ..."),
			exact: "invalid source_type: must be one of ...",
		},
		{
			name:  "wrapped validation",
			err:   fmt.Errorf("add: %w", app.NewValidationError("title", "is required")),
			exact: "invalid title: is required",
		},
		{
			name:  "not found",
			err:   fmt.Errorf("%w: evidence E-009 in INV-0001", app.ErrNotFound),
			exact: "not found: evidence E-009 in INV-0001",
		},
		{
			name: "no active",
			err:  app.ErrNoActiveInvestigation,
			has:  []string{app.ErrNoActiveInvestigation.Error(), "casebook_set_active_investigation", "investigation_ref"},
		},
		{
			name:  "conflict",
			err:   fmt.Errorf("%w: link crosses investigations", app.ErrConflict),
			exact: "conflict: link crosses investigations",
		},
		{
			name:  "busy",
			err:   app.ErrBusy,
			exact: app.ErrBusy.Error(),
		},
		{
			name:  "internal",
			err:   app.ErrInternal,
			exact: "casebook: internal storage error",
		},
		{
			name: "integrity",
			err:  &app.IntegrityError{Problems: []string{"integrity_check: damaged database pages"}, Total: 1},
			has:  []string{"integrity check failed", "damaged database pages"},
		},
		{
			name:  "cancelled",
			err:   context.Canceled,
			exact: "casebook: request cancelled",
		},
		{
			name:  "deadline",
			err:   fmt.Errorf("call: %w", context.DeadlineExceeded),
			exact: "casebook: request cancelled",
		},
		{
			name:  "other",
			err:   errors.New("open /tmp/x: permission denied"),
			exact: "open /tmp/x: permission denied",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := toolErrorText(t, toolError(c.err))
			if c.exact != "" && got != c.exact {
				t.Errorf("text = %q, want %q", got, c.exact)
			}
			for _, s := range c.has {
				if !strings.Contains(got, s) {
					t.Errorf("text %q does not contain %q", got, s)
				}
			}
		})
	}
}

func TestErrorClass(t *testing.T) {
	cases := map[string]error{
		"validation": app.NewValidationError("limit", "bad"),
		"not_found":  fmt.Errorf("%w: x", app.ErrNotFound),
		"no_active":  app.ErrNoActiveInvestigation,
		"conflict":   app.ErrConflict,
		"busy":       app.ErrBusy,
		"internal":   app.ErrInternal,
		"other":      errors.New("x"),
	}
	for want, err := range cases {
		if got := errorClass(err); got != want {
			t.Errorf("errorClass(%v) = %q, want %q", err, got, want)
		}
	}
}
