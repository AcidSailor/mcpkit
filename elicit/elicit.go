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

// GateID is the default key naming the write-tool confirmation request.
const GateID = "io.github.acidsailor.mcpkit/confirm"

// Ask builds the input-required result asking the client to confirm. It proves
// nothing about a call that already carries an answer; see the package doc.
func Ask(
	gateID string,
	session *mcp.ServerSession,
	params *mcp.ElicitParams,
) (*mcp.CallToolResult, error) {
	if session == nil {
		return nil, ErrNoElicitation
	}
	init := session.InitializeParams()
	if init == nil || init.Capabilities == nil ||
		init.Capabilities.Elicitation == nil {
		return nil, ErrNoElicitation
	}
	p := mcp.ElicitParams{}
	if params != nil {
		p = *params
	}
	if p.RequestedSchema == nil {
		p.RequestedSchema = emptyObject()
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{gateID: &p},
	}, nil
}

// Decide maps a fulfilled confirmation to nil (accept) or a sentinel error.
func Decide(resp mcp.InputResponse) error {
	res, ok := resp.(*mcp.ElicitResult)
	if !ok || res == nil {
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
