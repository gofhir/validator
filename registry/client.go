// Package registry provides a client for the FHIR Package Registry.
//
// The FHIR Package Registry (https://packages.fhir.org) hosts FHIR
// Implementation Guides and core packages. This client allows downloading
// packages for use in validation.
package registry

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultRegistryURL is the primary FHIR package registry.
	DefaultRegistryURL = "https://packages.fhir.org"

	// DefaultRegistry2URL is the secondary/mirror registry.
	DefaultRegistry2URL = "https://packages2.fhir.org"

	// DefaultTimeout for HTTP requests.
	DefaultTimeout = 30 * time.Second

	// DefaultCacheDir is the default location for cached packages.
	DefaultCacheDir = ".fhir/packages"

	// VersionLatest represents the "latest" version tag.
	VersionLatest = "latest"
)

// Client is a FHIR Package Registry client.
type Client struct {
	httpClient  *http.Client
	registryURL string
	cacheDir    string
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithRegistryURL sets a custom registry URL.
func WithRegistryURL(url string) ClientOption {
	return func(c *Client) {
		c.registryURL = url
	}
}

// WithCacheDir sets a custom cache directory.
func WithCacheDir(dir string) ClientOption {
	return func(c *Client) {
		c.cacheDir = dir
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithTimeout sets the HTTP timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// NewClient creates a new registry client.
func NewClient(opts ...ClientOption) *Client {
	// Get default cache directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	c := &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		registryURL: DefaultRegistryURL,
		cacheDir:    filepath.Join(homeDir, DefaultCacheDir),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// PackageInfo contains metadata about a package.
type PackageInfo struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	FHIRVersion  string            `json:"fhirVersion"`
	URL          string            `json:"url"`
	Canonical    string            `json:"canonical"`
	Dependencies map[string]string `json:"dependencies"`
}

// PackageManifest is the package.json in a FHIR package.
type PackageManifest struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	FHIRVersions []string          `json:"fhirVersions"`
	Dependencies map[string]string `json:"dependencies"`
	Author       string            `json:"author"`
	Canonical    string            `json:"canonical"`
	URL          string            `json:"url"`
	Title        string            `json:"title"`
	Type         string            `json:"type"`
}

// GetPackageInfo retrieves metadata about a package.
func (c *Client) GetPackageInfo(ctx context.Context, name, version string) (*PackageInfo, error) {
	// Get full package info (all versions)
	url := fmt.Sprintf("%s/%s", c.registryURL, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("package not found: %s (status %d)", name, resp.StatusCode)
	}

	var pkgInfo struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		DistTags    map[string]string `json:"dist-tags"`
		Versions    map[string]struct {
			Version     string `json:"version"`
			FHIRVersion string `json:"fhirVersion"`
			URL         string `json:"url"`
			Canonical   string `json:"canonical"`
		} `json:"versions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&pkgInfo); err != nil {
		return nil, fmt.Errorf("failed to decode package info: %w", err)
	}

	// Resolve version
	resolvedVersion := version
	if version == VersionLatest || version == "" {
		if latest, ok := pkgInfo.DistTags[VersionLatest]; ok {
			resolvedVersion = latest
		} else {
			return nil, fmt.Errorf("no latest version found for package %s", name)
		}
	}

	versionInfo, ok := pkgInfo.Versions[resolvedVersion]
	if !ok {
		return nil, fmt.Errorf("version %s not found for package %s", resolvedVersion, name)
	}

	return &PackageInfo{
		Name:        pkgInfo.Name,
		Version:     resolvedVersion,
		Description: pkgInfo.Description,
		FHIRVersion: versionInfo.FHIRVersion,
		URL:         versionInfo.URL,
		Canonical:   versionInfo.Canonical,
	}, nil
}

// DownloadPackage downloads and extracts a package to the cache directory.
// Returns the path to the extracted package.
func (c *Client) DownloadPackage(ctx context.Context, name, version string) (string, error) {
	// Resolve version if needed
	actualVersion := version
	if version == VersionLatest || version == "" {
		info, err := c.GetPackageInfo(ctx, name, VersionLatest)
		if err != nil {
			return "", err
		}
		actualVersion = info.Version
	}

	// Check if already cached
	packageDir := c.getPackagePath(name, actualVersion)
	if c.isPackageCached(packageDir) {
		return packageDir, nil
	}

	// Get package info to find tarball URL
	tarballURL, err := c.getTarballURL(ctx, name, actualVersion)
	if err != nil {
		return "", fmt.Errorf("failed to get tarball URL: %w", err)
	}

	// Download the package tarball
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tarballURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download package: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download package %s@%s: status %d", name, actualVersion, resp.StatusCode)
	}

	// Create cache directory
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Extract tarball
	if err := c.extractTarGz(resp.Body, packageDir); err != nil {
		// Clean up on failure
		os.RemoveAll(packageDir)
		return "", fmt.Errorf("failed to extract package: %w", err)
	}

	return packageDir, nil
}

// getTarballURL gets the download URL for a package version.
func (c *Client) getTarballURL(ctx context.Context, name, version string) (string, error) {
	url := fmt.Sprintf("%s/%s", c.registryURL, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("package not found: %s (status %d)", name, resp.StatusCode)
	}

	var pkgInfo struct {
		Versions map[string]struct {
			Dist struct {
				Tarball string `json:"tarball"`
			} `json:"dist"`
			URL string `json:"url"`
		} `json:"versions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&pkgInfo); err != nil {
		return "", fmt.Errorf("failed to decode package info: %w", err)
	}

	versionInfo, ok := pkgInfo.Versions[version]
	if !ok {
		return "", fmt.Errorf("version %s not found for package %s", version, name)
	}

	// Prefer dist.tarball, fallback to url
	if versionInfo.Dist.Tarball != "" {
		return versionInfo.Dist.Tarball, nil
	}
	if versionInfo.URL != "" {
		return versionInfo.URL, nil
	}

	return "", fmt.Errorf("no download URL found for %s@%s", name, version)
}

// GetPackage ensures a package is available locally (downloading if needed)
// and returns the path to it.
func (c *Client) GetPackage(ctx context.Context, name, version string) (string, error) {
	// Resolve "latest" version
	if version == VersionLatest || version == "" {
		info, err := c.GetPackageInfo(ctx, name, VersionLatest)
		if err != nil {
			return "", err
		}
		version = info.Version
	}

	return c.DownloadPackage(ctx, name, version)
}

// ReadManifest reads the package.json from a downloaded package.
func (c *Client) ReadManifest(packageDir string) (*PackageManifest, error) {
	manifestPath := filepath.Join(packageDir, "package", "package.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// Try without "package" subdirectory
		manifestPath = filepath.Join(packageDir, "package.json")
		data, err = os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read package.json: %w", err)
		}
	}

	var manifest PackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}

	return &manifest, nil
}

// ListCachedPackages returns all packages in the cache.
func (c *Client) ListCachedPackages() ([]string, error) {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var packages []string
	for _, entry := range entries {
		if entry.IsDir() {
			packages = append(packages, entry.Name())
		}
	}
	return packages, nil
}

// ClearCache removes all cached packages.
func (c *Client) ClearCache() error {
	return os.RemoveAll(c.cacheDir)
}

// CacheDir returns the cache directory path.
func (c *Client) CacheDir() string {
	return c.cacheDir
}

// getPackagePath returns the local path for a package.
func (c *Client) getPackagePath(name, version string) string {
	// Replace dots and slashes for safe directory names
	safeName := strings.ReplaceAll(name, "/", "-")
	return filepath.Join(c.cacheDir, fmt.Sprintf("%s#%s", safeName, version))
}

// isPackageCached checks if a package is already in the cache.
func (c *Client) isPackageCached(packageDir string) bool {
	// Check for package.json as indicator of valid package
	manifestPath := filepath.Join(packageDir, "package", "package.json")
	if _, err := os.Stat(manifestPath); err == nil {
		return true
	}
	manifestPath = filepath.Join(packageDir, "package.json")
	_, err := os.Stat(manifestPath)
	return err == nil
}

// extractTarGz extracts a tar.gz archive to a directory.
func (c *Client) extractTarGz(r io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Sanitize path to prevent directory traversal
		target := filepath.Join(destDir, header.Name) //nolint:gosec // G305: Path is validated below
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid tar path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Use 0644 for files as header.Mode may have unexpected values
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			// Limit file size to prevent decompression bombs (100MB max per file)
			const maxFileSize = 100 * 1024 * 1024
			if _, err := io.Copy(f, io.LimitReader(tr, maxFileSize)); err != nil {
				f.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			f.Close()
		}
	}

	return nil
}
