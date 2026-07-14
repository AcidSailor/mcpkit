package openapi_test

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/acidsailor/mcpkit/openapi"
)

// bigID is a real Alor order id (~2e18) that loses its low digits when passed
// through a float64 — the exact value the string codec must preserve.
const bigID = 2011734313187604080

func TestInt64String_UnmarshalString_PreservesBigID(t *testing.T) {
	var v openapi.Int64String
	require.NoError(t, json.Unmarshal([]byte(`"2011734313187604080"`), &v))
	assert.Equal(t, int64(bigID), v.Int64())
}

func TestInt64String_UnmarshalRejectsBareNumber(t *testing.T) {
	var v openapi.Int64String
	// A bare JSON number is already truncated by a float64 client; reject it
	// loudly rather than act on the wrong id.
	err := json.Unmarshal([]byte(`2011734313187604080`), &v)
	assert.Error(t, err)
}

func TestInt64String_UnmarshalRejectsInvalid(t *testing.T) {
	for _, in := range []string{`null`, `""`, `"12x"`, `"1.5"`, `" 12"`} {
		var v openapi.Int64String
		assert.Error(t, json.Unmarshal([]byte(in), &v), "input %s", in)
	}
}

func TestInt64String_MarshalEmitsQuotedString(t *testing.T) {
	v := openapi.Int64String(bigID)
	b, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, `"2011734313187604080"`, string(b))
}

func TestInt64String_RoundTripIsIdentity(t *testing.T) {
	const in = `"2011734313187604080"`
	var v openapi.Int64String
	require.NoError(t, json.Unmarshal([]byte(in), &v))
	out, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, in, string(out))
}

func TestInt64StringSchema(t *testing.T) {
	s := openapi.Int64StringSchema("the order id")
	assert.Equal(t, "string", s.Type)
	assert.NotEmpty(t, s.Pattern)
	assert.Equal(t, "the order id", s.Description)
}

func TestStringifyIntParam_RewritesAndPreservesDescription(t *testing.T) {
	s := openapi.Object(map[string]*jsonschema.Schema{
		"orderId": {Type: "integer", Description: "keep me"},
		"other":   {Type: "string"},
	}, []string{"orderId"})

	got := openapi.StringifyIntParam(s, "orderId")

	assert.Same(t, s, got, "returns the same schema for chaining")
	assert.Equal(t, "string", s.Properties["orderId"].Type)
	assert.Equal(t, "keep me", s.Properties["orderId"].Description)
	assert.Equal(t, "string", s.Properties["other"].Type, "other left alone")
}

func TestStringifyIntParam_AbsentNameIsNoOp(t *testing.T) {
	s := openapi.Object(map[string]*jsonschema.Schema{
		"other": {Type: "string"},
	}, nil)
	assert.NotPanics(t, func() { openapi.StringifyIntParam(s, "orderId") })
}
