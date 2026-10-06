package loader

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gofhir/validator/v2/internal/versionorder"
)

// DefaultRegistry is the FHIR package registry packages are downloaded from.
const DefaultRegistry = "https://packages.fhir.org"

// Limits on what a package registry is trusted with.
const (
	registryTimeout   = time.Minute     // a registry query
	downloadTimeout   = 5 * time.Minute // a package download
	maxPackageFile    = 1 << 30         // one file of a package archive
	maxPackageContent = 4 << 30         // all the files of a package archive
	staleInstallAge   = time.Hour       // an interrupted install's temporary directory
)

// validPackageID reports whether a package name or version can name a directory of the cache and a
// path of a registry: letters, digits, '.', '-', '_' and '+' (and the wildcards' '*'), not starting
// with '.'. Names and versions come from package manifests and registries, which are not trusted
// to stay inside the cache.
func validPackageID(s string) bool {
	if s == "" || s[0] == '.' {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '.', c == '-', c == '_', c == '+', c == '*':
		default:
			return false
		}
	}
	return !strings.Contains(s, "..")
}

func checkPackageID(name, version string) error {
	if !validPackageID(name) || !validPackageID(version) {
		return fmt.Errorf("invalid package %q#%q", name, version)
	}
	return nil
}

// Manifest reads the manifest (package.json) of a package in the cache.
func (l *Loader) Manifest(name, version string) (*PackageManifest, error) {
	if err := checkPackageID(name, version); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(l.packageDir(name, version), "package", "package.json"))
	if err != nil {
		return nil, fmt.Errorf("package %s#%s is not in the package cache %s: %w", name, version, l.basePath, err)
	}
	var m PackageManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("package %s#%s: failed to parse package manifest: %w", name, version, err)
	}
	return &m, nil
}

func (l *Loader) packageDir(name, version string) string {
	return filepath.Join(l.basePath, name+"#"+version)
}

// IsWildcard reports whether a version is a pattern with an "x" part ("3.3.x"), which package
// manifests use for the highest version matching it.
func IsWildcard(version string) bool {
	for part := range strings.SplitSeq(version, ".") {
		if part == "x" || part == "X" || part == "*" {
			return true
		}
	}
	return false
}

// versionMatches reports whether version matches pattern, part by part up to its wildcard. A
// pattern matches no pre-release ("3.3.1-ballot" for "3.3.x"), as npm version ranges do not.
func versionMatches(pattern, version string) bool {
	if strings.Contains(version, "-") {
		return false
	}
	ps, vs := strings.Split(pattern, "."), strings.Split(version, ".")
	for i, p := range ps {
		if p == "x" || p == "X" || p == "*" {
			return true
		}
		if i >= len(vs) || vs[i] != p {
			return false
		}
	}
	return len(ps) == len(vs)
}

// HighestMatching returns the highest of versions that pattern matches.
func HighestMatching(pattern string, versions []string) (string, bool) {
	best, found := "", false
	for _, v := range versions {
		if versionMatches(pattern, v) && (!found || versionorder.Less(best, v)) {
			best, found = v, true
		}
	}
	return best, found
}

// Satisfies reports whether one of the versions of a package loaded is the version a dependency
// declares, or matches it when it is a wildcard: the dependency needs nothing more loaded.
func Satisfies(loaded []string, declared string) bool {
	if IsWildcard(declared) {
		_, ok := HighestMatching(declared, loaded)
		return ok
	}
	return slices.Contains(loaded, declared)
}

// InstalledVersion resolves a version of a package against the cache: a wildcard to the highest
// version installed that matches it, any other version to itself when it is installed.
func (l *Loader) InstalledVersion(name, version string) (string, bool) {
	if checkPackageID(name, version) != nil {
		return "", false
	}
	if !IsWildcard(version) {
		return version, l.installed(name, version)
	}
	entries, err := os.ReadDir(l.basePath)
	if err != nil {
		return "", false
	}
	var versions []string
	for _, e := range entries {
		if n, v, ok := strings.Cut(e.Name(), "#"); ok && n == name && e.IsDir() && l.installed(n, v) {
			versions = append(versions, v)
		}
	}
	return HighestMatching(version, versions)
}

// installed reports whether the package is extracted in the cache.
func (l *Loader) installed(name, version string) bool {
	info, err := os.Stat(filepath.Join(l.packageDir(name, version), "package", "package.json"))
	return err == nil && info.Mode().IsRegular()
}

// registryURL is the URL of a package's document, or of one version of it, in a registry.
func registryURL(registry string, parts ...string) string {
	u := strings.TrimRight(registry, "/")
	for _, p := range parts {
		u += "/" + url.PathEscape(p)
	}
	return u
}

// PublishedVersion resolves a version of a package against a package registry: a wildcard to the
// highest version published that matches it, any other version to itself.
func PublishedVersion(ctx context.Context, registry, name, version string) (string, error) {
	if err := checkPackageID(name, version); err != nil {
		return "", err
	}
	if !IsWildcard(version) {
		return version, nil
	}
	ctx, cancel := context.WithTimeout(ctx, registryTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL(registry, name), http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("versions of %s: %s", name, resp.Status)
	}
	var doc struct {
		Versions map[string]json.RawMessage `json:"versions"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxPackageFile)).Decode(&doc); err != nil {
		return "", err
	}
	versions := make([]string, 0, len(doc.Versions))
	for v := range doc.Versions {
		if validPackageID(v) {
			versions = append(versions, v)
		}
	}
	v, ok := HighestMatching(version, versions)
	if !ok {
		return "", fmt.Errorf("no published version of %s matches %s", name, version)
	}
	return v, nil
}

// Install downloads a package from a package registry (such as [DefaultRegistry]) into the cache,
// unless it is there already, and reports whether it downloaded it. The version must not be a
// wildcard: see [PublishedVersion].
func (l *Loader) Install(ctx context.Context, registry, name, version string) (bool, error) {
	if err := checkPackageID(name, version); err != nil {
		return false, err
	}
	if IsWildcard(version) {
		return false, fmt.Errorf("install %s#%s: a wildcard version names no package", name, version)
	}
	if l.installed(name, version) {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	u := registryURL(registry, name, version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("download %s#%s: %w", name, version, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("download %s#%s from %s: %s", name, version, u, resp.Status)
	}
	if err := os.MkdirAll(l.basePath, 0o750); err != nil {
		return false, err
	}
	l.removeStaleInstalls()
	// Extract into a temporary sibling and rename, so a partial download never looks installed.
	tmp, err := os.MkdirTemp(l.basePath, ".install-*")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := extractTgz(resp.Body, tmp); err != nil {
		return false, fmt.Errorf("download %s#%s: %w", name, version, err)
	}
	if err := os.Rename(tmp, l.packageDir(name, version)); err != nil {
		if l.installed(name, version) {
			return false, nil // installed meanwhile, by another process
		}
		return false, err
	}
	return true, nil
}

// removeStaleInstalls removes the temporary directories of installs interrupted long ago, which
// would otherwise stay in a cache other tools share.
func (l *Loader) removeStaleInstalls() {
	entries, err := os.ReadDir(l.basePath)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), ".install-") {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > staleInstallAge {
			_ = os.RemoveAll(filepath.Join(l.basePath, e.Name()))
		}
	}
}

// extractTgz extracts a package archive into dst. It refuses paths that leave dst, and files and
// archives larger than the limits; links are not extracted.
func extractTgz(r io.Reader, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	var total int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(hdr.Name))
		if name == "." || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) || filepath.IsAbs(name) {
			return fmt.Errorf("unsafe path in archive: %q", hdr.Name)
		}
		target := filepath.Join(dst, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return err
			}
		case tar.TypeReg:
			if hdr.Size > maxPackageFile {
				return fmt.Errorf("file %q in archive is larger than %d bytes", hdr.Name, int64(maxPackageFile))
			}
			if total += hdr.Size; total > maxPackageContent {
				return fmt.Errorf("archive is larger than %d bytes", int64(maxPackageContent))
			}
			if err := extractFile(tr, target, hdr.Size); err != nil {
				return err
			}
		}
	}
}

// extractFile writes the size bytes of the current archive entry to target.
func extractFile(tr io.Reader, target string, size int64) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(f, tr, size); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
