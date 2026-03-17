package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
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

// ProgressFunc is called after each individual download completes.
// It receives the 0-based index, total count, and the result.
// Implementations must be safe for concurrent use.
type ProgressFunc func(index int, total int, result DownloadResult)

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

// WithProgressFunc sets a callback that is invoked after each download completes.
func WithProgressFunc(fn ProgressFunc) Option {
	return func(d *Downloader) {
		d.progressFunc = fn
	}
}

// Downloader orchestrates downloading Chef packages to a local directory.
type Downloader struct {
	dest         string
	product      string
	concurrency  int
	skipExisting bool
	httpClient   *http.Client
	progressFunc ProgressFunc
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

	total := len(packages)
	for i, pkg := range packages {
		wg.Add(1)
		go func(idx int, p chefapi.FlatPackage) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = d.downloadOne(ctx, p)
			if d.progressFunc != nil {
				d.progressFunc(idx, total, results[idx])
			}
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

	// Check skip-existing using a SHA256 marker file for the directory.
	// We need the filename to check, but we may not know it until after
	// the HTTP redirect. For skip-existing, check if any .sha256 file
	// in the directory matches the expected checksum.
	if d.skipExisting {
		if matches, _ := filepath.Glob(filepath.Join(dir, "*.sha256")); len(matches) > 0 {
			for _, m := range matches {
				if existing, err := os.ReadFile(m); err == nil {
					if strings.TrimSpace(string(existing)) == pkg.SHA256 {
						result.Path = strings.TrimSuffix(m, ".sha256")
						result.Skipped = true
						return result
					}
				}
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

	// Determine filename: prefer Content-Disposition header, fall back to URL path
	filename := filenameFromContentDisposition(resp.Header.Get("Content-Disposition"))
	if filename == "" {
		filename = filenameFromURL(resp.Request.URL.String())
	}
	result.Path = filepath.Join(dir, filename)
	shaPath := result.Path + ".sha256"

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

// filenameFromContentDisposition extracts a filename from a Content-Disposition
// header value (e.g. `attachment; filename="chef-ice-19.2.12-1.amzn2.x86_64.rpm"`).
// Returns an empty string if the header is missing, empty, or has no filename parameter.
func filenameFromContentDisposition(header string) string {
	if header == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	name := params["filename"]
	if name == "" || name == "." || name == "/" {
		return ""
	}
	// Sanitise: take only the base name to avoid directory traversal
	return filepath.Base(name)
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
	if base == "." || base == "/" || !strings.Contains(base, ".") {
		// No file extension — likely a generic endpoint name like "download".
		// Try to build a meaningful name from query params instead.
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
		if base != "." && base != "/" {
			return base
		}
		return "download"
	}
	return base
}
