package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

func testServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
}

func testPackage(url, sha256, version, platform, platVer, arch string) chefapi.FlatPackage {
	return chefapi.FlatPackage{
		Platform:        platform,
		PlatformVersion: platVer,
		Architecture:    arch,
		PackageDetail: chefapi.PackageDetail{
			URL:     url,
			SHA256:  sha256,
			Version: version,
		},
	}
}

func sha256sum(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func TestDownload_Success(t *testing.T) {
	body := "fake package content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, "chef", WithConcurrency(1))

	pkg := testPackage(server.URL+"/chef.rpm", checksum, "18.4.12", "el", "9", "x86_64")
	results, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Err != nil {
		t.Errorf("unexpected download error: %v", r.Err)
	}
	if r.Skipped {
		t.Error("expected not skipped")
	}

	// Verify file exists in correct directory structure
	expectedDir := filepath.Join(dest, "chef", "18.4.12", "el", "9", "x86_64")
	entries, err := os.ReadDir(expectedDir)
	if err != nil {
		t.Fatalf("expected directory %s to exist: %v", expectedDir, err)
	}
	// Should have the package file and .sha256 sidecar
	fileCount := 0
	shaCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sha256") {
			shaCount++
		} else {
			fileCount++
		}
	}
	if fileCount != 1 {
		t.Errorf("expected 1 package file, got %d", fileCount)
	}
	if shaCount != 1 {
		t.Errorf("expected 1 sha256 sidecar, got %d", shaCount)
	}

	// Verify SHA256 sidecar content
	shaPath := r.Path + ".sha256"
	shaContent, err := os.ReadFile(shaPath)
	if err != nil {
		t.Fatalf("reading sha256 sidecar: %v", err)
	}
	if !strings.Contains(string(shaContent), checksum) {
		t.Errorf("sha256 sidecar should contain %s, got %s", checksum, string(shaContent))
	}
}

func TestDownload_SHA256Mismatch(t *testing.T) {
	body := "fake package content"
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, "chef", WithConcurrency(1))

	pkg := testPackage(server.URL+"/chef.rpm", "wrong_checksum", "18.4.12", "el", "9", "x86_64")
	results, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err == nil {
		t.Error("expected SHA256 mismatch error")
	}
}

func TestDownload_SkipExisting(t *testing.T) {
	body := "fake package content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, "chef", WithConcurrency(1), WithSkipExisting(true))

	pkg := testPackage(server.URL+"/chef.rpm", checksum, "18.4.12", "el", "9", "x86_64")

	// First download
	results1, _ := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if results1[0].Skipped {
		t.Error("first download should not be skipped")
	}

	// Second download should be skipped
	results2, _ := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if !results2[0].Skipped {
		t.Error("second download should be skipped")
	}
}

func TestDownload_SkipExistingDisabled(t *testing.T) {
	body := "fake package content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, "chef", WithConcurrency(1), WithSkipExisting(false))

	pkg := testPackage(server.URL+"/chef.rpm", checksum, "18.4.12", "el", "9", "x86_64")

	// First download
	d.Download(context.Background(), []chefapi.FlatPackage{pkg})

	// Second download should NOT be skipped
	results2, _ := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if results2[0].Skipped {
		t.Error("download should not be skipped when skip-existing is disabled")
	}
}

func TestDownload_ConcurrentMultiple(t *testing.T) {
	body := "fake package content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, "chef", WithConcurrency(4))

	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/a.rpm", checksum, "18.4.12", "el", "9", "x86_64"),
		testPackage(server.URL+"/b.rpm", checksum, "18.4.12", "el", "8", "x86_64"),
		testPackage(server.URL+"/c.deb", checksum, "18.4.12", "ubuntu", "22.04", "x86_64"),
		testPackage(server.URL+"/d.deb", checksum, "18.4.12", "debian", "12", "x86_64"),
	}

	results, err := d.Download(context.Background(), packages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result %d: unexpected error: %v", i, r.Err)
		}
	}
}

func TestDownload_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, "chef", WithConcurrency(1))

	pkg := testPackage(server.URL+"/missing.rpm", "abc", "18.4.12", "el", "9", "x86_64")
	results, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err == nil {
		t.Error("expected HTTP error")
	}
}

func TestDownload_ContextCancelled(t *testing.T) {
	server := testServer(t, "content")
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, "chef", WithConcurrency(1))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	pkg := testPackage(server.URL+"/chef.rpm", "abc", "18.4.12", "el", "9", "x86_64")
	results, err := d.Download(ctx, []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err == nil {
		t.Error("expected context cancelled error")
	}
}

func TestDownload_FilenameFromURL(t *testing.T) {
	body := "fake package content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, "chef", WithConcurrency(1))

	pkg := testPackage(server.URL+"/path/to/chef-18.4.12-1.el9.x86_64.rpm", checksum, "18.4.12", "el", "9", "x86_64")
	results, _ := d.Download(context.Background(), []chefapi.FlatPackage{pkg})

	if results[0].Err != nil {
		t.Fatalf("unexpected error: %v", results[0].Err)
	}
	basename := filepath.Base(results[0].Path)
	if basename != "chef-18.4.12-1.el9.x86_64.rpm" {
		t.Errorf("expected filename chef-18.4.12-1.el9.x86_64.rpm, got %s", basename)
	}
}

func TestFilenameFromContentDisposition(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		// Standard attachment with quoted filename
		{`attachment; filename="chef-ice-19.2.12-1.amzn2.x86_64.rpm"`, "chef-ice-19.2.12-1.amzn2.x86_64.rpm"},
		// Unquoted filename
		{`attachment; filename=chef-ice-19.2.12-1.amzn2.x86_64.rpm`, "chef-ice-19.2.12-1.amzn2.x86_64.rpm"},
		// With filename* (RFC 5987) — mime.ParseMediaType decodes this into "filename"
		{`attachment; filename="chef-18.10.17-1.el10.x86_64.rpm"; filename*=UTF-8''chef-18.10.17-1.el10.x86_64.rpm`, "chef-18.10.17-1.el10.x86_64.rpm"},
		// Inline disposition
		{`inline; filename="report.pdf"`, "report.pdf"},
		// Empty header
		{"", ""},
		// No filename parameter
		{"attachment", ""},
		// Filename is just a dot
		{`attachment; filename="."`, ""},
		// Filename is just a slash
		{`attachment; filename="/"`, ""},
		// Directory traversal attempt — sanitised to base name
		{`attachment; filename="../../../etc/passwd"`, "passwd"},
		// Filename with path component — sanitised to base name
		{`attachment; filename="path/to/file.deb"`, "file.deb"},
	}
	for _, tt := range tests {
		got := filenameFromContentDisposition(tt.header)
		if got != tt.want {
			t.Errorf("filenameFromContentDisposition(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestDownload_ContentDispositionPreferred(t *testing.T) {
	// Simulate a server that returns the file directly (no redirect) with a
	// Content-Disposition header — like the chef-ice download endpoint.
	// The URL path ends in "/download" (a bad filename) but the header
	// provides the real name.
	body := "fake chef-ice package"
	checksum := sha256sum(body)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="chef-ice-19.2.12-1.amzn2.x86_64.rpm"`)
		w.Write([]byte(body))
	}))
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, "chef-ice", WithConcurrency(1))

	// URL path is /current/chef-ice/download — filepath.Base would give "download"
	pkg := testPackage(server.URL+"/current/chef-ice/download?p=linux&m=x86_64&pm=rpm&v=19.2.12",
		checksum, "19.2.12", "linux", "x86_64", "rpm")
	results, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err != nil {
		t.Fatalf("unexpected download error: %v", results[0].Err)
	}

	basename := filepath.Base(results[0].Path)
	if basename != "chef-ice-19.2.12-1.amzn2.x86_64.rpm" {
		t.Errorf("expected filename from Content-Disposition header, got %q", basename)
	}
}

func TestDownload_FallsBackToURLWhenNoContentDisposition(t *testing.T) {
	// Server returns no Content-Disposition header — filename should come from URL path
	body := "fake package"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, "chef", WithConcurrency(1))

	pkg := testPackage(server.URL+"/files/chef-18.4.12-1.el9.x86_64.rpm",
		checksum, "18.4.12", "el", "9", "x86_64")
	results, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err != nil {
		t.Fatalf("unexpected download error: %v", results[0].Err)
	}

	basename := filepath.Base(results[0].Path)
	if basename != "chef-18.4.12-1.el9.x86_64.rpm" {
		t.Errorf("expected filename from URL path, got %q", basename)
	}
}

// Suppress unused import warnings
var _ = fmt.Sprintf
var _ = json.Marshal
