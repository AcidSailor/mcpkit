package elicit

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	accept  = "accept"
	decline = "decline"
	cancel  = "cancel"
)

// GateID names the input request carrying the write-tool confirmation.
const GateID = "io.github.acidsailor.mcpkit/confirm"

// Ask builds the input-required result asking the client to confirm.
func Ask(
	session *mcp.ServerSession,
	params *mcp.ElicitParams,
) (*mcp.CallToolResult, error) {
	init := session.InitializeParams()
	if init == nil || init.Capabilities == nil ||
		init.Capabilities.Elicitation == nil {
		return nil, ErrNoElicitation
	}
	if params == nil {
		params = &mcp.ElicitParams{}
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{GateID: params},
	}, nil
}

// Response returns the confirmation the client fulfilled, if present.
func Response(responses mcp.InputResponseMap) (mcp.InputResponse, bool) {
	resp, ok := responses[GateID]
	return resp, ok
}

// Decide maps a fulfilled confirmation to nil (accept) or a sentinel error.
func Decide(resp mcp.InputResponse) error {
	res, ok := resp.(*mcp.ElicitResult)
	if !ok {
		return fmt.Errorf("%w: got %T", ErrElicitationFailed, resp)
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
