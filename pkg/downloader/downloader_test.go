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

// Suppress unused import warnings
var _ = fmt.Sprintf
var _ = json.Marshal
