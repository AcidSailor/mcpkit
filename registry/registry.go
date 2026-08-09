package registry

import (
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Access classifies a registration and determines whether Bind gates it.
type Access int

const (
	AccessRead Access = iota
	AccessWrite
	// AccessResource marks a read-only resource or template; it always binds.
	AccessResource
)

// Registration describes one tool or resource without a server.
type Registration struct {
	Name   string
	Access Access
	bind   func(*mcp.Server)
}

// Enable selects which registrations Bind installs; Write gates writes.
type Enable struct {
	Write bool
}

// Registry is an ordered, server-independent collection of registrations.
type Registry []Registration

// New flattens registration groups into a Registry and preserves order.
func New(groups ...[]Registration) Registry {
	return slices.Concat(groups...)
}

// Bind installs enabled registrations onto s; writes skipped unless en.Write.
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
