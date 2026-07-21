package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirm_AutoApprove(t *testing.T) {
	out := &bytes.Buffer{}
	if !Confirm(out, strings.NewReader(""), "proceed?", true) {
		t.Error("autoApprove=true should confirm without reading input")
	}
	if out.Len() != 0 {
		t.Errorf("autoApprove=true should not prompt, got output %q", out.String())
	}
}

func TestConfirm_Interactive(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"YES\n", true},
		{"n\n", false},
		{"\n", false},
		{"anything\n", false},
	}
	for _, c := range cases {
		out := &bytes.Buffer{}
		got := Confirm(out, strings.NewReader(c.input), "proceed?", false)
		if got != c.want {
			t.Errorf("Confirm with input %q = %v, want %v", c.input, got, c.want)
		}
		if !strings.Contains(out.String(), "proceed?") {
			t.Errorf("expected prompt to be printed, got %q", out.String())
		}
	}
}
