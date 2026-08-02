package handler

import "testing"

func TestExtractVersionPreservesSemVerSuffixes(t *testing.T) {
	tests := map[string]string{
		"v0.1.2-alpha.4":                    "0.1.2-alpha.4",
		"Release V1.2.3-rc.1+windows.amd64": "1.2.3-rc.1+windows.amd64",
		"ginnungagap_v0.0.4":                "0.0.4",
	}
	for input, want := range tests {
		if got := extractVersion(input); got != want {
			t.Errorf("extractVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCompareVersionsSemVerPrecedence(t *testing.T) {
	ascending := []string{
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
	}
	for i := 1; i < len(ascending); i++ {
		cur, latest := ascending[i-1], ascending[i]
		if !compareVersions(cur, latest) {
			t.Errorf("expected %q to be newer than %q", latest, cur)
		}
		if compareVersions(latest, cur) {
			t.Errorf("did not expect %q to be newer than %q", cur, latest)
		}
	}
}

func TestCompareVersionsApplicationCases(t *testing.T) {
	tests := []struct {
		name        string
		cur, latest string
		want        bool
	}{
		{name: "next alpha", cur: "0.1.2-alpha.3", latest: "0.1.2-alpha.4", want: true},
		{name: "alpha to stable", cur: "0.1.2-alpha.4", latest: "0.1.2", want: true},
		{name: "next core prerelease", cur: "0.1.2", latest: "0.1.3-alpha.1", want: true},
		{name: "build metadata ignored", cur: "1.2.3+build.1", latest: "1.2.3+build.2", want: false},
		{name: "v prefix", cur: "v2.0.0-rc.1", latest: "V2.0.0", want: true},
		{name: "large numeric identifier", cur: "1.0.0-alpha.99999999999999999999", latest: "1.0.0-alpha.100000000000000000000", want: true},
		{name: "invalid current", cur: "1.0", latest: "1.0.1", want: false},
		{name: "invalid leading zero", cur: "1.0.0-alpha.01", latest: "1.0.0-alpha.2", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareVersions(tt.cur, tt.latest); got != tt.want {
				t.Fatalf("compareVersions(%q, %q) = %v, want %v", tt.cur, tt.latest, got, tt.want)
			}
		})
	}
}
