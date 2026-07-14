package openapi_test

import (
	"encoding/json"
	"math"
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
	// Any bare JSON number is rejected — the guard is unconditional, not
	// range-driven: a small in-range 12 is refused just like a huge id that a
	// float64 client has already truncated.
	for _, in := range []string{`12`, `2011734313187604080`} {
		var v openapi.Int64String
		assert.Error(t, json.Unmarshal([]byte(in), &v), "input %s", in)
	}
}

func TestInt64String_UnmarshalRejectsInvalid(t *testing.T) {
	// The pattern (^-?[0-9]+$) is looser than the decoder by design: an
	// overflowing digit run matches the pattern yet ParseInt rejects it. Both
	// int64 bounds+1 must fail loud.
	invalid := []string{
		`null`, `""`, `"12x"`, `"1.5"`, `" 12"`,
		`"9223372036854775808"`,  // math.MaxInt64 + 1
		`"-9223372036854775809"`, // math.MinInt64 - 1
	}
	for _, in := range invalid {
		var v openapi.Int64String
		assert.Error(t, json.Unmarshal([]byte(in), &v), "input %s", in)
	}
}

func TestInt64String_RoundTripBoundariesAndNegative(t *testing.T) {
	cases := map[string]int64{
		`"9223372036854775807"`:  math.MaxInt64,
		`"-9223372036854775808"`: math.MinInt64,
		`"2011734313187604080"`:  bigID,
		`"-2011734313187604080"`: -bigID,
		`"0"`:                    0,
	}
	for in, want := range cases {
		var v openapi.Int64String
		require.NoError(t, json.Unmarshal([]byte(in), &v), "input %s", in)
		assert.Equal(t, want, v.Int64(), "input %s", in)
		out, err := json.Marshal(v)
		require.NoError(t, err, "input %s", in)
		assert.JSONEq(t, in, string(out), "input %s", in)
	}
}

func TestInt64StringSchema(t *testing.T) {
	s := openapi.Int64StringSchema("the order id")
	assert.Equal(t, "string", s.Type)
	assert.NotEmpty(t, s.Pattern)
	assert.Equal(t, "the order id", s.Description)
}

func TestStringifyIntParam_RewritesAndPreservesTitleDescription(t *testing.T) {
	s := openapi.Object(map[string]*jsonschema.Schema{
		"orderId": {Type: "integer", Title: "Order id", Description: "keep me"},
		"other":   {Type: "string"},
	}, []string{"orderId"})

	got := openapi.StringifyIntParam(s, "orderId")

	assert.Same(t, s, got, "returns the same schema for chaining")
	assert.Equal(t, "string", s.Properties["orderId"].Type)
	assert.Equal(t, "Order id", s.Properties["orderId"].Title)
	assert.Equal(t, "keep me", s.Properties["orderId"].Description)
	assert.Equal(t, "string", s.Properties["other"].Type, "other left alone")
}

func TestStringifyIntParam_AbsentNamePanics(t *testing.T) {
	s := openapi.Object(map[string]*jsonschema.Schema{
		"other": {Type: "string"},
	}, nil)
	assert.PanicsWithError(
		t,
		`openapi undefined: property "orderId"`,
		func() { openapi.StringifyIntParam(s, "orderId") },
	)
}
