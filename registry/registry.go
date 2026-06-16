package registry

import (
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Access classifies a tool as read-only or state-mutating; Bind gates
// AccessWrite tools behind Enable.Write.
type Access int

const (
	AccessRead Access = iota
	AccessWrite
)

// Registration is a server-independent description of one tool. bind captures
// the toolkit.New(...).AddRead/AddWrite call, deferring the *mcp.Server to Bind.
type Registration struct {
	Name   string
	Access Access
	bind   func(*mcp.Server)
}

// Enable selects which registrations Bind installs; Write gates AccessWrite
// tools.
type Enable struct {
	Write bool
}

// Registry is an ordered, server-independent collection of registrations.
type Registry []Registration

// New flattens tool-group slices into a Registry, preserving order.
func New(groups ...[]Registration) Registry {
	return slices.Concat(groups...)
}

// Bind installs the enabled registrations onto s. AccessWrite is skipped
// unless en.Write is true; AccessRead always binds.
func (r Registry) Bind(s *mcp.Server, en Enable) {
	for _, reg := range r {
		if reg.Access == AccessWrite && !en.Write {
			continue
		}
		if reg.bind == nil {
			panic("registry: Registration " + reg.Name +
				" has no bind func; construct it with Read or Write")
		}
		reg.bind(s)
	}
}
