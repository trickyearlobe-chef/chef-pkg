package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

// DownloadResult holds the outcome of a single package download.
type DownloadResult struct {
	Path    string              // Local file path
	Package chefapi.FlatPackage // The package that was downloaded
	Skipped bool                // True if skipped due to existing file
	Err     error               // Non-nil if download failed
}

// Option is a functional option for configuring a Downloader.
type Option func(*Downloader)

// WithConcurrency sets the max number of parallel downloads.
func WithConcurrency(n int) Option {
	return func(d *Downloader) {
		if n > 0 {
			d.concurrency = n
		}
	}
}

// WithSkipExisting enables or disables skipping already-downloaded files.
func WithSkipExisting(skip bool) Option {
	return func(d *Downloader) {
		d.skipExisting = skip
	}
}

// WithHTTPClient sets a custom HTTP client for downloads.
func WithHTTPClient(hc *http.Client) Option {
	return func(d *Downloader) {
		d.httpClient = hc
	}
}

// Downloader orchestrates downloading Chef packages to a local directory.
type Downloader struct {
	dest         string
	product      string
	concurrency  int
	skipExisting bool
	httpClient   *http.Client
}

// New creates a new Downloader.
func New(dest, product string, opts ...Option) *Downloader {
	d := &Downloader{
		dest:         dest,
		product:      product,
		concurrency:  4,
		skipExisting: true,
		httpClient:   &http.Client{},
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Download fetches all given packages concurrently and returns the results.
// Individual download failures are captured in DownloadResult.Err rather than
// failing the entire batch.
func (d *Downloader) Download(ctx context.Context, packages []chefapi.FlatPackage) ([]DownloadResult, error) {
	results := make([]DownloadResult, len(packages))
	sem := make(chan struct{}, d.concurrency)
	var wg sync.WaitGroup

	for i, pkg := range packages {
		wg.Add(1)
		go func(idx int, p chefapi.FlatPackage) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = d.downloadOne(ctx, p)
		}(i, pkg)
	}

	wg.Wait()
	return results, nil
}

// downloadOne handles downloading a single package.
func (d *Downloader) downloadOne(ctx context.Context, pkg chefapi.FlatPackage) DownloadResult {
	result := DownloadResult{Package: pkg}

	// Build the target directory: {dest}/{product}/{version}/{platform}/{platform_version}/{arch}/
	dir := filepath.Join(d.dest, d.product, pkg.Version, pkg.Platform, pkg.PlatformVersion, pkg.Architecture)
	filename := filenameFromURL(pkg.URL)
	result.Path = filepath.Join(dir, filename)
	shaPath := result.Path + ".sha256"

	// Skip if file exists and SHA256 matches
	if d.skipExisting {
		if existing, err := os.ReadFile(shaPath); err == nil {
			if strings.TrimSpace(string(existing)) == pkg.SHA256 {
				result.Skipped = true
				return result
			}
		}
	}

	// Create directory structure
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Err = fmt.Errorf("creating directory %s: %w", dir, err)
		return result
	}

	// Download the file
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.URL, nil)
	if err != nil {
		result.Err = fmt.Errorf("creating request: %w", err)
		return result
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		result.Err = fmt.Errorf("downloading %s: %w", pkg.URL, err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Err = fmt.Errorf("downloading %s: HTTP %d", pkg.URL, resp.StatusCode)
		return result
	}

	// Write to a temp file, compute SHA256 as we go
	tmpFile, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		result.Err = fmt.Errorf("creating temp file: %w", err)
		return result
	}
	tmpPath := tmpFile.Name()
	defer func() {
		// Clean up temp file on error
		if result.Err != nil {
			os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		tmpFile.Close()
		result.Err = fmt.Errorf("writing file: %w", err)
		return result
	}
	tmpFile.Close()

	// Verify SHA256
	gotSHA := hex.EncodeToString(hasher.Sum(nil))
	if pkg.SHA256 != "" && gotSHA != pkg.SHA256 {
		os.Remove(tmpPath)
		result.Err = fmt.Errorf("SHA256 mismatch for %s: expected %s, got %s", filename, pkg.SHA256, gotSHA)
		return result
	}

	// Move temp file to final location
	if err := os.Rename(tmpPath, result.Path); err != nil {
		result.Err = fmt.Errorf("moving file to %s: %w", result.Path, err)
		return result
	}

	// Write SHA256 sidecar file
	if err := os.WriteFile(shaPath, []byte(gotSHA+"\n"), 0644); err != nil {
		result.Err = fmt.Errorf("writing sha256 sidecar: %w", err)
		return result
	}

	return result
}

// filenameFromURL extracts a filename from a download URL.
// If the URL has a path component, the last segment is used.
// Otherwise, falls back to the full URL path.
func filenameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "download"
	}
	path := u.Path
	if path == "" || path == "/" {
		// Try to build a name from query params
		q := u.Query()
		parts := []string{}
		if p := q.Get("p"); p != "" {
			parts = append(parts, p)
		}
		if v := q.Get("v"); v != "" {
			parts = append(parts, v)
		}
		if m := q.Get("m"); m != "" {
			parts = append(parts, m)
		}
		if pm := q.Get("pm"); pm != "" {
			parts = append(parts, pm)
		}
		if len(parts) > 0 {
			return strings.Join(parts, "-")
		}
		return "download"
	}
	base := filepath.Base(path)
	if base == "." || base == "/" {
		return "download"
	}
	return base
}
