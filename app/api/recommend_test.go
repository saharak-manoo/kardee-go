package api

import (
	"slices"
	"testing"
)

func TestParseCards_NormalizesAndDedups(t *testing.T) {
	got := parseCards(" TTB-So-Smart ,ttb-so-smart,,ktc-x")
	if !slices.Equal(got, []string{"ttb-so-smart", "ktc-x"}) {
		t.Errorf("got %v", got)
	}
}

func TestParseAmount(t *testing.T) {
	if n, given, err := parseAmount(""); n != 0 || given || err != nil {
		t.Errorf("empty → %d %v %v", n, given, err)
	}
	if n, given, err := parseAmount("1500"); n != 1500 || !given || err != nil {
		t.Errorf("1500 → %d %v %v", n, given, err)
	}
	for _, bad := range []string{"0", "-5", "abc", "1.5"} {
		if _, _, err := parseAmount(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
