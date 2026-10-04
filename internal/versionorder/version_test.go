package versionorder

import "testing"

func TestLess(t *testing.T) {
	ordered := []string{"", "1", "2018-08-12", "2019-01-01", "R4", "0.9", "0.11.0", "0.22.0", "1.0", "1.0.0-ballot", "1.0.0-ballot2", "1.0.0", "1.0.1", "1.2",
		"1.10.0", "2.0.0-snapshot1", "2.0.0", "5.3.0-ballot", "5.3.0", "5.9.0", "5.10.0", "10.0.0"}
	for i := range ordered {
		for j := range ordered {
			if got, want := Less(ordered[i], ordered[j]), i < j; got != want {
				t.Errorf("Less(%q, %q) = %v, want %v", ordered[i], ordered[j], got, want)
			}
		}
	}
}

// The order is total and transitive whatever the versions: every triple agrees.
func TestLessIsATotalOrder(t *testing.T) {
	vs := []string{"", "1", "9", "10", "1a", "01", "2018-08-12", "R4", "1.0", "1.0.0", "1.0.0+b1", "1.0.0+b2",
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.01"}
	for _, a := range vs {
		if Less(a, a) {
			t.Errorf("Less(%q, %q)", a, a)
		}
		for _, b := range vs {
			if a != b && Less(a, b) == Less(b, a) {
				t.Errorf("%q and %q are not ordered", a, b)
			}
			for _, c := range vs {
				if Less(a, b) && Less(b, c) && !Less(a, c) {
					t.Errorf("Less(%q, %q) and Less(%q, %q), but not Less(%q, %q)", a, b, b, c, a, c)
				}
			}
		}
	}
	// Semver 11's example.
	ordered := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0"}
	for i := 1; i < len(ordered); i++ {
		if !Less(ordered[i-1], ordered[i]) {
			t.Errorf("Less(%q, %q) is false", ordered[i-1], ordered[i])
		}
	}
}
