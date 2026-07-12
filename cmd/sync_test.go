package cmd

import "testing"

func TestIndent(t *testing.T) {
	if got := indent("a\nb\nc", "  "); got != "  a\n  b\n  c\n" {
		t.Errorf("indent = %q", got)
	}
	if got := indent("single", "> "); got != "> single\n" {
		t.Errorf("indent single = %q", got)
	}
}