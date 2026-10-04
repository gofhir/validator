package validator

import (
	"context"
	"fmt"
	"sort"

	"github.com/gofhir/validator/pkg/loader"
	"github.com/gofhir/validator/pkg/logger"
)

// loadDependencies loads, transitively, the packages the given ones depend on, in the versions
// they declare: a guide's definitions resolve against those of the packages it was built with. A
// dependency in a version already loaded is not loaded again (nor looked for); in another version,
// it is loaded too, and each canonical resolves as the registry tells (see
// registry.Registry.SetFHIRVersion).
//
// One core package is loaded, the one for the FHIR version validated: a dependency on the core
// package of another FHIR version is reported and not loaded. A dependency missing from the package
// cache is downloaded from config.PackageRegistry when one is set, and otherwise reported and not
// loaded.
func loadDependencies(l *loader.Loader, config *Config, loaded, from []*loader.Package) []*loader.Package {
	have := map[string][]string{} // name -> versions loaded
	tried := map[string]bool{}    // "name#version" declared, loaded or not
	coreLoaded := false
	for _, pkg := range loaded {
		have[pkg.Name] = append(have[pkg.Name], pkg.Version)
		coreLoaded = coreLoaded || pkg.IsCore()
	}

	type dependency struct{ name, version, of string }
	var queue []dependency
	enqueue := func(pkg *loader.Package) {
		names := make([]string, 0, len(pkg.Dependencies))
		for name := range pkg.Dependencies {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			queue = append(queue, dependency{name, pkg.Dependencies[name], pkg.Name + "#" + pkg.Version})
		}
	}
	for _, pkg := range from {
		enqueue(pkg)
	}

	var added []*loader.Package
	for len(queue) > 0 {
		dep := queue[0]
		queue = queue[1:]
		if loader.Satisfies(have[dep.name], dep.version) || tried[dep.name+"#"+dep.version] {
			continue
		}
		tried[dep.name+"#"+dep.version] = true
		version, err := dependencyVersion(l, config, dep.name, dep.version)
		if err != nil {
			logger.Warn("Dependency %s#%s of %s is not loaded: %v", dep.name, dep.version, dep.of, err)
			continue
		}
		id := dep.name + "#" + version
		if loader.Satisfies(have[dep.name], version) || (version != dep.version && tried[id]) {
			continue
		}
		tried[id] = true
		manifest, err := l.Manifest(dep.name, version)
		if err != nil {
			logger.Warn("Dependency %s of %s is not loaded: %v", id, dep.of, err)
			continue
		}
		if manifest.IsCore() {
			if !manifest.IsFor(config.FHIRVersion) {
				logger.Warn("Dependency %s of %s is the core package of FHIR %s, not of the version validated, %s: not loaded",
					id, dep.of, coreVersion(manifest), config.FHIRVersion)
				continue
			}
			if coreLoaded {
				continue // the core package of the version validated is loaded
			}
			coreLoaded = true
		}
		pkg, err := l.LoadPackage(dep.name, version)
		if err != nil {
			logger.Warn("Dependency %s of %s is not loaded: %v", id, dep.of, err)
			continue
		}
		logger.Info("  Loaded dependency %s of %s", id, dep.of)
		have[dep.name] = append(have[dep.name], version)
		added = append(added, pkg)
		enqueue(pkg)
	}
	return added
}

// dependencyVersion resolves the version of a package to load against the cache, downloading it
// from config.PackageRegistry when it is not there and a registry is set. It fails, telling why,
// when the package is not available.
func dependencyVersion(l *loader.Loader, config *Config, name, version string) (string, error) {
	if v, ok := l.InstalledVersion(name, version); ok {
		return v, nil
	}
	if config.PackageRegistry == "" {
		return "", fmt.Errorf("it is not in the package cache %s, and no package registry is set to download it from", l.BasePath())
	}
	ctx := context.Background()
	v, err := loader.PublishedVersion(ctx, config.PackageRegistry, name, version)
	if err != nil {
		return "", err
	}
	downloaded, err := l.Install(ctx, config.PackageRegistry, name, v)
	if err != nil {
		return "", err
	}
	if downloaded {
		logger.Info("  Downloaded %s#%s from %s", name, v, config.PackageRegistry)
	}
	return v, nil
}

// addedPackages leaves out of the packages added those loaded already, at the same version, and
// the core packages of another FHIR version than the one validated, reporting the latter: one core
// package is loaded, the version validated's.
func addedPackages(loaded, added []*loader.Package, fhirVersion string) []*loader.Package {
	have := map[string]bool{}
	for _, pkg := range loaded {
		have[pkg.Name+"#"+pkg.Version] = true
	}
	kept := added[:0]
	for _, pkg := range added {
		id := pkg.Name + "#" + pkg.Version
		switch {
		case have[id]:
			continue
		case pkg.IsCore() && !pkg.IsFor(fhirVersion) && pkg.Version != fhirVersion:
			logger.Warn("Package %s is the core package of another FHIR version than the one validated, %s: not loaded", id, fhirVersion)
			continue
		}
		have[id] = true
		kept = append(kept, pkg)
	}
	return kept
}

// coreVersion is the FHIR version a core package is for.
func coreVersion(m *loader.PackageManifest) string {
	if len(m.FHIRVersions) > 0 {
		return m.FHIRVersions[0]
	}
	if m.FHIRVersion != "" {
		return m.FHIRVersion
	}
	return m.Version
}
