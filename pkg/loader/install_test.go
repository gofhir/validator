package loader

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWildcardVersions(t *testing.T) {
	v, ok := HighestMatching("3.3.x", []string{"3.2.0", "3.3.0", "3.3.10", "3.3.2", "3.4.0"})
	if !ok || v != "3.3.10" {
		t.Errorf("3.3.x -> %q, %v; want 3.3.10", v, ok)
	}
	if v, _ := HighestMatching("3.3.x", []string{"3.3.0", "3.3.1-ballot"}); v != "3.3.0" {
		t.Errorf("3.3.x -> %q; want 3.3.0, not a pre-release", v)
	}
	if _, ok := HighestMatching("3.5.x", []string{"3.3.0"}); ok {
		t.Error("no match must be reported")
	}
	if !versionMatches("1.0.0", "1.0.0") || versionMatches("1.0.0", "1.0.1") || IsWildcard("1.0.0") || !IsWildcard("3.x") {
		t.Error("exact versions must match only themselves")
	}
}

// A package whose archive names a path outside the package directory is refused.
func TestExtractRefusesPathsOutside(t *testing.T) {
	for _, name := range []string{"../evil.json", "/abs.json", "package/../../evil.json"} {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: 2, Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte("{}"))
		_ = tw.Close()
		_ = gz.Close()
		err := extractTgz(&buf, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "unsafe path") {
			t.Errorf("%s: err = %v, want an unsafe path refused", name, err)
		}
	}
}

func archive(t *testing.T, entries ...*tar.Header) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range entries {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write(bytes.Repeat([]byte("x"), int(h.Size))); err != nil {
				t.Fatal(err)
			}
		}
	}
	_ = tw.Close()
	_ = gz.Close()
	return &buf
}

// Links in an archive are not extracted, and a file larger than the limit is refused.
func TestExtractLinksAndSizes(t *testing.T) {
	dst := t.TempDir()
	buf := archive(t,
		&tar.Header{Name: "package/link.json", Linkname: "/etc/passwd", Typeflag: tar.TypeSymlink},
		&tar.Header{Name: "package/hard.json", Linkname: "/etc/passwd", Typeflag: tar.TypeLink},
		&tar.Header{Name: "package/a.json", Mode: 0o600, Size: 2, Typeflag: tar.TypeReg})
	if err := extractTgz(buf, dst); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"link.json", "hard.json"} {
		if _, err := os.Lstat(filepath.Join(dst, "package", name)); err == nil {
			t.Errorf("%s was extracted", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dst, "package", "a.json")); err != nil {
		t.Errorf("a.json: %v", err)
	}

	// The header's size is checked before anything is written.
	var big bytes.Buffer
	gz := gzip.NewWriter(&big)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "package/big.json", Mode: 0o600, Size: maxPackageFile + 1, Typeflag: tar.TypeReg})
	_ = gz.Flush()
	if err := extractTgz(&big, t.TempDir()); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("err = %v, want the file refused for its size", err)
	}
}

// Names and versions that would leave the cache, or a registry path, are refused.
func TestInvalidPackageIDs(t *testing.T) {
	l := NewLoader(t.TempDir())
	for _, id := range [][2]string{{"a", "1/../../x"}, {"../a", "1.0.0"}, {"a", ".."}, {"a/b", "1.0.0"}, {"", "1"}, {"a", "1 0"}} {
		if _, err := l.Manifest(id[0], id[1]); err == nil || !strings.Contains(err.Error(), "invalid package") {
			t.Errorf("Manifest(%q, %q) = %v", id[0], id[1], err)
		}
		if _, err := l.Install(context.Background(), "http://127.0.0.1:1", id[0], id[1]); err == nil || !strings.Contains(err.Error(), "invalid package") {
			t.Errorf("Install(%q, %q) = %v", id[0], id[1], err)
		}
		if _, ok := l.InstalledVersion(id[0], id[1]); ok {
			t.Errorf("InstalledVersion(%q, %q) is installed", id[0], id[1])
		}
	}
}

// A wildcard resolves against installed packages only: a file named like a package is not one.
func TestInstalledVersionIgnoresFiles(t *testing.T) {
	cache := t.TempDir()
	if err := os.WriteFile(filepath.Join(cache, "acme#1.0.9.tgz"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(cache, "acme#1.0.2", "package")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cache, "acme#1.0.5"), 0o750); err != nil { // no manifest
		t.Fatal(err)
	}
	if v, ok := NewLoader(cache).InstalledVersion("acme", "1.0.x"); !ok || v != "1.0.2" {
		t.Errorf("InstalledVersion = %q, %v; want 1.0.2", v, ok)
	}
}
