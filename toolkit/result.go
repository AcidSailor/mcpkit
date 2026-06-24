package toolkit

import "encoding/json"

// Envelopes wrap a bare slice or scalar so the JSON root stays an object.
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

// MarshalJSON normalizes a nil slice to "items":[] rather than null.
func (i Items[T]) MarshalJSON() ([]byte, error) {
	items := i.Items
	if items == nil {
		items = []T{}
	}
	return json.Marshal(struct {
		Items []T `json:"items"`
	}{Items: items})
}

// WrapItems adapts a (slice, error) pair into an Items envelope.
func WrapItems[T any](v []T, err error) (Items[T], error) {
	return Items[T]{Items: v}, err
}

// WrapValue adapts a (scalar, error) pair into a Value envelope.
func WrapValue[T any](v T, err error) (Value[T], error) {
	return Value[T]{Value: v}, err
}
