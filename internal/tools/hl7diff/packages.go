package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/specs"
)

// PackageID is a FHIR package reference, "id#version".
type PackageID struct{ Name, Version string }

func (p PackageID) String() string { return p.Name + "#" + p.Version }

// ParsePackageID parses "id#version"; both parts are required.
func ParsePackageID(s string) (PackageID, error) {
	name, ver, ok := strings.Cut(s, "#")
	if !ok || name == "" || ver == "" {
		return PackageID{}, fmt.Errorf("package %q: want id#version", s)
	}
	return PackageID{name, ver}, nil
}

// Cache is the standard FHIR package cache (~/.fhir/packages), where packages are extracted
// directories named "<id>#<version>", as the HL7 validator installs them.
type Cache struct{ Dir string }

// DefaultCache is the cache pkg/loader reads, created if it does not exist yet.
func DefaultCache() (Cache, error) {
	dir := loader.DefaultPackagePath()
	if dir == "" {
		return Cache{}, errors.New("cannot locate the FHIR package cache: no home directory")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return Cache{}, err
	}
	return Cache{dir}, nil
}

// Fingerprint identifies a package directory's contents without reading them: every file's path,
// size and modification time. A package replaced under the same id#version changes it.
func (c Cache) Fingerprint(p PackageID) (string, error) {
	h := sha256.New()
	root := c.Path(p)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Index files are written by tools that read the package (the HL7 validator writes
		// .index.json and .index.db); they are not the package's content.
		if d.IsDir() || strings.HasPrefix(d.Name(), ".index") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		_, _ = fmt.Fprintln(h, rel, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}

// Path is the package's directory in the cache.
func (c Cache) Path(p PackageID) string { return filepath.Join(c.Dir, p.String()) }

type packageJSON struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}

func (c Cache) manifest(p PackageID) (packageJSON, error) {
	var m packageJSON
	data, err := os.ReadFile(filepath.Join(c.Path(p), "package", "package.json"))
	if err != nil {
		return m, fmt.Errorf("package %s is not in the cache %s (hl7diff fetch %s): %w", p, c.Dir, p, err)
	}
	err = json.Unmarshal(data, &m)
	return m, err
}

// EmbeddedNames are the packages gofhir embeds for a FHIR version, name to version, read from the
// embedded packages themselves. A dependency closure leaves out an embedded package at that
// version or an older one, and keeps a newer one: an IG that pins definitions of the newer version
// needs them loaded, as the HL7 validator loads them, and the registry keeps each version apart by
// url|version.
func EmbeddedNames(fhirVersion string) (map[string]string, error) {
	names := map[string]string{}
	for _, tgz := range specs.GetPackages(fhirVersion) {
		m, err := tgzManifest(tgz)
		if err != nil {
			return nil, fmt.Errorf("embedded package for %s: %w", fhirVersion, err)
		}
		names[m.Name] = m.Version
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("gofhir embeds no packages for FHIR %s", fhirVersion)
	}
	return names, nil
}

func tgzManifest(tgz []byte) (packageJSON, error) {
	var m packageJSON
	gz, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		return m, err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return m, errors.New("no package/package.json")
		}
		if err != nil {
			return m, err
		}
		if filepath.ToSlash(hdr.Name) == "package/package.json" {
			err := json.NewDecoder(tr).Decode(&m)
			return m, err
		}
	}
}

// Closure returns the given packages and everything they depend on, transitively, minus the
// packages gofhir embeds at the same or an older version (skipped, returned separately so the
// report can say so). Every package in the closure must be in the cache.
func (c Cache) Closure(roots []PackageID, embedded map[string]string) (closure, skipped []PackageID, err error) {
	seen := map[string]bool{}
	var p PackageID
	queue := append([]PackageID(nil), roots...)
	for len(queue) > 0 {
		p = queue[0]
		queue = queue[1:]
		if ev, isEmbedded := embedded[p.Name]; isEmbedded {
			installed, err := c.resolveInstalled(p)
			if err != nil || !versionLess(ev, installed.Version) {
				if !seen[p.String()] {
					seen[p.String()] = true
					skipped = append(skipped, p)
				}
				continue
			}
			p = installed
		} else if p, err = c.resolveInstalled(p); err != nil {
			return nil, nil, err
		}
		if seen[p.String()] {
			continue
		}
		seen[p.String()] = true
		m, err := c.manifest(p)
		if err != nil {
			return nil, nil, err
		}
		closure = append(closure, p)
		deps := make([]string, 0, len(m.Dependencies))
		for name := range m.Dependencies {
			deps = append(deps, name)
		}
		sort.Strings(deps)
		for _, name := range deps {
			queue = append(queue, PackageID{name, m.Dependencies[name]})
		}
	}
	// gofhir's registry resolves a canonical to one definition, so two versions of one package
	// would overwrite each other in load order. Keep the highest version of each name.
	best := map[string]PackageID{}
	for _, p := range closure {
		if cur, ok := best[p.Name]; !ok || versionLess(cur.Version, p.Version) {
			best[p.Name] = p
		}
	}
	kept := closure[:0]
	for _, p := range closure {
		if best[p.Name] == p {
			kept = append(kept, p)
		} else {
			skipped = append(skipped, p)
		}
	}
	closure = kept
	sort.Slice(closure, func(a, b int) bool { return closure[a].String() < closure[b].String() })
	sort.Slice(skipped, func(a, b int) bool { return skipped[a].String() < skipped[b].String() })
	return closure, skipped, nil
}

// versionLess orders versions as semver does: dotted parts numerically where they are numbers
// ("5.10.0" > "5.9.0"), and a pre-release before its release ("5.3.0-ballot" < "5.3.0").
func versionLess(a, b string) bool {
	amain, apre, _ := strings.Cut(a, "-")
	bmain, bpre, _ := strings.Cut(b, "-")
	if amain != bmain {
		return dottedLess(amain, bmain)
	}
	switch {
	case apre == bpre:
		return false
	case apre == "":
		return false // a release is newer than any of its pre-releases
	case bpre == "":
		return true
	}
	return dottedLess(apre, bpre)
}

func dottedLess(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		an, aerr := strconv.Atoi(as[i])
		bn, berr := strconv.Atoi(bs[i])
		if aerr == nil && berr == nil {
			return an < bn
		}
		return as[i] < bs[i]
	}
	return len(as) < len(bs)
}

// Examples lists the example instances in a package directory of the cache: sub is where the
// package keeps them ("package/example" in IGs; "package" in an examples-only package such as
// hl7.fhir.r4.examples). The package manifest and index files are not instances.
func (c Cache) Examples(p PackageID, sub string) ([]string, error) {
	if sub == "" {
		sub = "package/example"
	}
	dir := filepath.Join(c.Path(p), filepath.FromSlash(sub))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("examples of %s: %w", p, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), ".") && e.Name() != "package.json" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("package %s has no examples in %s", p, dir)
	}
	return files, nil
}

// Fetch downloads a package from the FHIR package registry into the cache, unless it is there.
func (c Cache) Fetch(ctx context.Context, p PackageID, registry string) (bool, error) {
	if _, err := c.manifest(p); err == nil {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(registry, "/")+"/"+p.Name+"/"+p.Version, http.NoBody)
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("fetch %s: %s", p, resp.Status)
	}
	// Extract into a temporary sibling and rename, so a partial download never looks installed.
	tmp, err := os.MkdirTemp(c.Dir, ".fetch-*")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := extractTgz(resp.Body, tmp); err != nil {
		return false, fmt.Errorf("fetch %s: %w", p, err)
	}
	if err := os.Rename(tmp, c.Path(p)); err != nil {
		return false, err
	}
	return true, nil
}

func extractTgz(r io.Reader, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(hdr.Name))
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("unsafe path in archive: %q", hdr.Name)
		}
		target := filepath.Join(dst, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return err
			}
			// Packages are a few hundred MB at most; cap a single file to stop a bomb.
			if _, err := io.Copy(f, io.LimitReader(tr, 1<<30)); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
}

// FetchClosure installs the packages and, transitively, their dependencies, including those gofhir
// embeds: the HL7 validator loads them from the cache. It returns what it downloaded.
func (c Cache) FetchClosure(ctx context.Context, roots []PackageID, registry string) ([]PackageID, error) {
	var got []PackageID
	seen := map[string]bool{}
	queue := append([]PackageID(nil), roots...)
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		p, err := c.resolveInstalled(p)
		if err != nil {
			if p, err = resolvePublished(ctx, p, registry); err != nil {
				return got, err
			}
		}
		if seen[p.String()] {
			continue
		}
		seen[p.String()] = true
		fetched, err := c.Fetch(ctx, p, registry)
		if err != nil {
			return got, err
		}
		if fetched {
			got = append(got, p)
		}
		m, err := c.manifest(p)
		if err != nil {
			return got, err
		}
		for name, ver := range m.Dependencies {
			queue = append(queue, PackageID{name, ver})
		}
	}
	return got, nil
}

// isWildcard reports a version pattern with an "x" part ("3.3.x"), which package manifests use
// for "the highest matching version".
func isWildcard(v string) bool {
	for part := range strings.SplitSeq(v, ".") {
		if part == "x" || part == "X" || part == "*" {
			return true
		}
	}
	return false
}

func versionMatches(pattern, v string) bool {
	ps, vs := strings.Split(pattern, "."), strings.Split(v, ".")
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

func highestMatching(pattern string, versions []string) (string, bool) {
	best, found := "", false
	for _, v := range versions {
		if versionMatches(pattern, v) && (!found || versionLess(best, v)) {
			best, found = v, true
		}
	}
	return best, found
}

// resolveInstalled turns a wildcard version into the highest installed match.
func (c Cache) resolveInstalled(p PackageID) (PackageID, error) {
	if !isWildcard(p.Version) {
		return p, nil
	}
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return p, err
	}
	var versions []string
	for _, e := range entries {
		if name, ver, ok := strings.Cut(e.Name(), "#"); ok && name == p.Name {
			versions = append(versions, ver)
		}
	}
	v, ok := highestMatching(p.Version, versions)
	if !ok {
		return p, fmt.Errorf("no installed version of %s matches %s (hl7diff fetch)", p.Name, p.Version)
	}
	return PackageID{p.Name, v}, nil
}

// resolvePublished turns a wildcard version into the highest published match.
func resolvePublished(ctx context.Context, p PackageID, registry string) (PackageID, error) {
	if !isWildcard(p.Version) {
		return p, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(registry, "/")+"/"+p.Name, http.NoBody)
	if err != nil {
		return p, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return p, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return p, fmt.Errorf("versions of %s: %s", p.Name, resp.Status)
	}
	var doc struct {
		Versions map[string]json.RawMessage `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return p, err
	}
	versions := make([]string, 0, len(doc.Versions))
	for v := range doc.Versions {
		versions = append(versions, v)
	}
	v, ok := highestMatching(p.Version, versions)
	if !ok {
		return p, fmt.Errorf("no published version of %s matches %s", p.Name, p.Version)
	}
	return PackageID{p.Name, v}, nil
}
