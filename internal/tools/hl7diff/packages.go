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
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofhir/validator/v2/internal/versionorder"
	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/specs"
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

type packageJSON = loader.PackageManifest

func (c Cache) manifest(p PackageID) (*packageJSON, error) {
	var m packageJSON
	data, err := os.ReadFile(filepath.Join(c.Path(p), "package", "package.json"))
	if err != nil {
		return nil, fmt.Errorf("package %s is not in the cache %s (hl7diff fetch %s): %w", p, c.Dir, p, err)
	}
	err = json.Unmarshal(data, &m)
	return &m, err
}

// EmbeddedNames are the packages gofhir embeds for a FHIR version, name to version, read from the
// embedded packages themselves.
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

// Closure returns the given packages and everything they depend on, transitively, in every version
// they are depended on in, as gofhir loads them (loader.Satisfies decides whether a dependency
// needs anything more loaded): minus the base packages it runs with and core packages, which the
// base provides for the FHIR version validated (left out, returned separately so the report can
// say so). Every package in the closure must be in the cache.
func (c Cache) Closure(roots []PackageID, base map[string]string) (closure, skipped []PackageID, err error) {
	have := map[string][]string{}
	for name, version := range base {
		have[name] = append(have[name], version)
	}
	seen := map[string]bool{}
	queue := append([]PackageID(nil), roots...)
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if v, isBase := base[p.Name]; isBase && loader.Satisfies([]string{v}, p.Version) {
			if id := (PackageID{p.Name, v}); !seen[id.String()] {
				seen[id.String()] = true
				skipped = append(skipped, id)
			}
			continue
		}
		if loader.Satisfies(have[p.Name], p.Version) {
			continue
		}
		if p, err = c.resolveInstalled(p); err != nil {
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
		if m.IsCore() {
			skipped = append(skipped, p)
			continue
		}
		have[p.Name] = append(have[p.Name], p.Version)
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
	sort.Slice(closure, func(a, b int) bool { return closure[a].String() < closure[b].String() })
	sort.Slice(skipped, func(a, b int) bool { return skipped[a].String() < skipped[b].String() })
	return closure, skipped, nil
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
	return loader.NewLoader(c.Dir).Install(ctx, registry, p.Name, p.Version)
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

// resolveInstalled turns a wildcard version into the highest installed match.
func (c Cache) resolveInstalled(p PackageID) (PackageID, error) {
	v, ok := loader.NewLoader(c.Dir).InstalledVersion(p.Name, p.Version)
	if !ok {
		if loader.IsWildcard(p.Version) {
			return p, fmt.Errorf("no installed version of %s matches %s (hl7diff fetch)", p.Name, p.Version)
		}
		return p, nil // the manifest read reports it missing
	}
	return PackageID{p.Name, v}, nil
}

// resolvePublished turns a wildcard version into the highest published match.
func resolvePublished(ctx context.Context, p PackageID, registry string) (PackageID, error) {
	v, err := loader.PublishedVersion(ctx, registry, p.Name, p.Version)
	return PackageID{p.Name, v}, err
}

// ParsePackageSummary reads the packages the HL7 validator loaded from its output: the line
// "Package Summary: [id#ver, id#ver, ...]". It fails when the output has none.
func ParsePackageSummary(out string) ([]PackageID, error) {
	const marker = "Package Summary: ["
	i := strings.LastIndex(out, marker)
	if i < 0 {
		return nil, errors.New("the HL7 validator's output names no packages (no Package Summary)")
	}
	rest := out[i+len(marker):]
	j := strings.IndexByte(rest, ']')
	if j < 0 {
		return nil, errors.New("the HL7 validator's Package Summary is not closed")
	}
	var ids []PackageID
	for _, s := range splitList(rest[:j]) {
		id, err := ParsePackageID(s)
		if err != nil {
			return nil, fmt.Errorf("the Package Summary: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// packageFamily is a package's name without its FHIR-version flavor: the same package published
// for several FHIR versions (hl7.terminology.r4, hl7.terminology.r5, hl7.terminology) is one family,
// with the same content in each flavor.
func packageFamily(name string) string {
	for _, flavor := range []string{".r3", ".r4b", ".r4", ".r5"} {
		if base, ok := strings.CutSuffix(name, flavor); ok {
			return base
		}
	}
	return name
}

// EffectiveBase returns the base packages for gofhir to load so that it uses the versions the HL7
// validator uses: for each package gofhir embeds (embedded: name to version), the highest version
// the HL7 validator loaded of its family, in any flavor (it resolves a canonical to the latest
// version among all the packages it loads), under gofhir's name for it. A family the HL7 validator
// did not load keeps gofhir's embedded version.
func EffectiveBase(embedded map[string]string, hl7Loaded []PackageID) []PackageID {
	names := make([]string, 0, len(embedded))
	for n := range embedded {
		names = append(names, n)
	}
	sort.Strings(names)
	base := make([]PackageID, 0, len(names))
	for _, n := range names {
		v := ""
		for _, p := range hl7Loaded {
			if packageFamily(p.Name) == packageFamily(n) && (v == "" || versionorder.Less(v, p.Version)) {
				v = p.Version
			}
		}
		if v == "" {
			v = embedded[n]
		}
		base = append(base, PackageID{n, v})
	}
	return base
}
