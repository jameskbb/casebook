package mcpserver

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
)

// toolError maps an error to a tool error result (IsError true) per
// docs/api.md section 6. It is the only place tool failures are worded.
func toolError(err error) *mcp.CallToolResult {
	res := &mcp.CallToolResult{}
	res.SetError(errors.New(errorText(err)))
	return res
}

// errorText is the text a client sees for err. App errors are safe to show
// (raw storage text never leaves internal/app); internal failures and
// cancellation get fixed wording.
func errorText(err error) string {
	if ve, ok := app.IsValidation(err); ok {
		return "invalid " + ve.Field + ": " + ve.Message
	}
	switch {
	case errors.Is(err, app.ErrNoActiveInvestigation):
		return err.Error() + ". Call casebook_list_investigations and casebook_set_active_investigation, or pass investigation_ref."
	case errors.Is(err, app.ErrNotFound), errors.Is(err, app.ErrConflict), errors.Is(err, app.ErrBusy):
		return err.Error()
	case isIntegrity(err):
		return err.Error()
	case errors.Is(err, app.ErrInternal):
		return "casebook: internal storage error"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "casebook: request cancelled"
	default:
		return err.Error()
	}
}

func isIntegrity(err error) bool {
	_, ok := app.IsIntegrity(err)
	return ok
}
