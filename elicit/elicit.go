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

// Gate runs write-tool elicitation: accept->nil, else a sentinel error.
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
