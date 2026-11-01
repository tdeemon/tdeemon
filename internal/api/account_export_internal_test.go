package api

import "testing"

// Text a spreadsheet would take for a formula goes into a CSV file after
// a quote; numbers and other text as they are.
func TestCSVValue(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		in   any
		want string
	}{
		{nil, ""},
		{int64(-5), "-5"},
		{-0.25, "-0.25"},
		{"Rye bread", "Rye bread"},
		{"", ""},
		{`=HYPERLINK("https://example.com")`, `'=HYPERLINK("https://example.com")`},
		{"+1", "'+1"},
		{"-1+1", "'-1+1"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"\tcmd", "'\tcmd"},
		{"\rcmd", "'\rcmd"},
	} {
		if got := csvValue(c.in); got != c.want {
			t.Errorf("csvValue(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
