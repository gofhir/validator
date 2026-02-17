package specs

import "testing"

func TestHasVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"4.0.1", true},
		{"4.3.0", true},
		{"5.0.0", true},
		{"3.0.2", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := HasVersion(tt.version); got != tt.want {
			t.Errorf("HasVersion(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestGetPackages(t *testing.T) {
	tests := []struct {
		version string
		wantLen int
		wantNil bool
	}{
		{"4.0.1", 3, false},
		{"4.3.0", 3, false},
		{"5.0.0", 3, false},
		{"3.0.2", 0, true},
	}
	for _, tt := range tests {
		pkgs := GetPackages(tt.version)
		if tt.wantNil {
			if pkgs != nil {
				t.Errorf("GetPackages(%q) = non-nil, want nil", tt.version)
			}
			continue
		}
		if len(pkgs) != tt.wantLen {
			t.Errorf("GetPackages(%q) returned %d packages, want %d", tt.version, len(pkgs), tt.wantLen)
		}
	}
}

func TestEmbeddedIsValidGzip(t *testing.T) {
	allPackages := []struct {
		name string
		data []byte
	}{
		{"r4Core", r4Core},
		{"terminologyR4", terminologyR4},
		{"extensionsR4", extensionsR4},
		{"r4bCore", r4bCore},
		{"r5Core", r5Core},
		{"terminologyR5", terminologyR5},
		{"extensionsR5", extensionsR5},
	}
	for _, pkg := range allPackages {
		if len(pkg.data) < 2 {
			t.Errorf("%s: data too short (%d bytes)", pkg.name, len(pkg.data))
			continue
		}
		// Gzip magic bytes: 0x1f 0x8b
		if pkg.data[0] != 0x1f || pkg.data[1] != 0x8b {
			t.Errorf("%s: invalid gzip header: got [%#x %#x], want [0x1f 0x8b]", pkg.name, pkg.data[0], pkg.data[1])
		}
	}
}

func TestSharedPackages(t *testing.T) {
	r4 := GetPackages("4.0.1")
	r4b := GetPackages("4.3.0")

	// R4 and R4B should share the same terminology package (index 1)
	if &r4[1][0] != &r4b[1][0] {
		t.Error("R4 and R4B should share the same terminologyR4 data")
	}

	// R4 and R4B should share the same extensions package (index 2)
	if &r4[2][0] != &r4b[2][0] {
		t.Error("R4 and R4B should share the same extensionsR4 data")
	}
}
