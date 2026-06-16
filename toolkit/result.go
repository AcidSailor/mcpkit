package toolkit

import "encoding/json"

// MCP requires structuredContent's root to be a JSON object, but many APIs
// return a bare array or scalar. Since AddRead/AddWrite marshal Out as-is (no
// auto-wrap), a handler returning a slice or scalar wraps it in one of these
// envelopes. Pair with an output schema whose root is an object with an "items"
// (array) or "value" property.
type (
	// Items wraps a slice result under the "items" key.
	Items[T any] struct {
		Items []T `json:"items"`
	}
	// Value wraps a scalar result under the "value" key.
	Value[T any] struct {
		Value T `json:"value"`
	}
)

// MarshalJSON normalizes a nil slice to "items":[] rather than null (which an
// array-typed schema rejects), for every construction path including the zero
// value, not only WrapItems.
func (i Items[T]) MarshalJSON() ([]byte, error) {
	items := i.Items
	if items == nil {
		items = []T{}
	}
	return json.Marshal(struct {
		Items []T `json:"items"`
	}{Items: items})
}

// WrapItems adapts a (slice, error) pair — e.g. WrapItems(client.List(ctx)) —
// into an Items envelope. Items.MarshalJSON handles nil-slice normalization, so
// the result always marshals to a JSON array.
func WrapItems[T any](v []T, err error) (Items[T], error) {
	return Items[T]{Items: v}, err
}

// WrapValue adapts a (scalar, error) pair into a Value envelope.
func WrapValue[T any](v T, err error) (Value[T], error) {
	return Value[T]{Value: v}, err
}
