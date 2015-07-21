package requestlog

import (
	"testing"
)

func TestEscape(t *testing.T) {
	var l line

	testcases := []struct {
		i, o string
	}{
		{"", ""},
		{"simplestring", "simplestring"},
		{"string with spaces", `string\ with\ spaces`},
	}
	for _, tc := range testcases {
		l.Reset()
		l.WriteEscaped(tc.i)
		if l.String() != tc.o {
			t.Error("Unexpected result '%s' for '%s': expecting '%s'", l.String(), tc.i, tc.o)
		}
	}
}
