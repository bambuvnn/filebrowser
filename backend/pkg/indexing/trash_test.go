package indexing

import "testing"

func TestIsTrashIndexPath(t *testing.T) {
	cases := map[string]bool{
		"/.trash":           true,
		"/.trash/":          true,
		".trash":            true,
		"/.trash/1/a.txt":   true,
		"/.trashy":          false,
		"/docs/.trash":      false,
		"/docs/.trash/a":    false,
		"/":                 false,
		"":                  false,
		"/my.trash/a":       false,
	}
	for in, want := range cases {
		if got := IsTrashIndexPath(in); got != want {
			t.Errorf("IsTrashIndexPath(%q) = %v, want %v", in, got, want)
		}
	}
}
