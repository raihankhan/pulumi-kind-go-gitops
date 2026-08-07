package cluster

import (
	"testing"
)

// TestExistsMatchesNames validates the cluster name parsing logic against the
// exact kind CLI output format. The helper is intentionally small so that
// the wider provider can grow without dragging test surface along.
func TestExistsMatchesNames(t *testing.T) {
	cases := []struct {
		name    string
		listing string
		target  string
		want    bool
	}{
		{"hit", "alpha\nbeta\ngamma", "alpha", true},
		{"miss", "alpha\nbeta", "gamma", false},
		{"exact", "pulumi-gitops", "pulumi-gitops", true},
		{"padding", "  pulumi-gitops  \n", "pulumi-gitops", true},
		{"empty", "", "missing", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			found := false
			for _, line := range rangeLines(tc.listing) {
				if trim(line) == tc.target {
					found = true
					break
				}
			}
			if found != tc.want {
				t.Fatalf("got %v, want %v", found, tc.want)
			}
		})
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		out = append(out, s[start:])
	}
	return out
}

func rangeLines(s string) []string {
	return splitLines(s)
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
