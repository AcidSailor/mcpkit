package elicit

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	accept  = "accept"
	decline = "decline"
	cancel  = "cancel"
)

// Gate runs the MCP elicitation handshake for a write tool. It requires the
// client to advertise the elicitation capability (else ErrNoElicitation), then
// issues the request and maps the action to a result:
//
//	accept  -> nil (caller proceeds with the mutating call)
//	decline -> ErrUserDeclined
//	cancel  -> ErrUserCanceled
//	other   -> ErrUnexpectedElicitAction
//
// A transport/protocol failure is wrapped with ErrElicitationFailed. A nil
// params produces an empty prompt.
//
// Only the action gates the call; any returned field values are not
// inspected.
func Gate(
	ctx context.Context,
	session *mcp.ServerSession,
	params *mcp.ElicitParams,
) error {
	init := session.InitializeParams()
	if init == nil || init.Capabilities == nil ||
		init.Capabilities.Elicitation == nil {
		return ErrNoElicitation
	}

	if params == nil {
		params = &mcp.ElicitParams{}
	}
	res, err := session.Elicit(ctx, params)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrElicitationFailed, err)
	}

	switch res.Action {
	case accept:
		return nil
	case decline:
		return ErrUserDeclined
	case cancel:
		return ErrUserCanceled
	default:
		return fmt.Errorf("%w: %q", ErrUnexpectedElicitAction, res.Action)
	}
}
