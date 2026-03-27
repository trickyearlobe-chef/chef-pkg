package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

func testServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
}

func testPackage(url, sha256Hash, product, version, platform, platVer, arch string) chefapi.FlatPackage {
	return chefapi.FlatPackage{
		Product:         product,
		Platform:        platform,
		PlatformVersion: platVer,
		Architecture:    arch,
		PackageDetail: chefapi.PackageDetail{
			URL:     url,
			SHA256:  sha256Hash,
			Version: version,
		},
	}
}

func sha256sum(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// ---------- Phase 2a: Platform-first directory layout ----------

func TestDownload_PlatformFirstLayout(t *testing.T) {
	body := "fake package content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	pkg := testPackage(server.URL+"/chef.rpm", checksum, "chef", "18.4.12", "el", "9", "x86_64")
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

	// Verify file exists in platform-first directory structure:
	// {dest}/{platform}/{platform_version}/{arch}/{product}/{version}/
	expectedDir := filepath.Join(dest, "el", "9", "x86_64", "chef", "18.4.12")
	entries, err := os.ReadDir(expectedDir)
	if err != nil {
		t.Fatalf("expected directory %s to exist: %v", expectedDir, err)
	}
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

func TestDownload_PlatformFirstLayout_MultipleProducts(t *testing.T) {
	body := "fake package content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	// Disable dedup — both packages share the same body/checksum
	d := New(dest, WithConcurrency(1), WithDedup(false))

	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/chef.rpm", checksum, "chef", "18.4.12", "el", "9", "x86_64"),
		testPackage(server.URL+"/inspec.rpm", checksum, "inspec", "6.8.24", "el", "9", "x86_64"),
	}

	results, err := d.Download(context.Background(), packages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both products should exist under same platform path
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error for %s: %v", r.Package.Product, r.Err)
		}
	}

	// Verify chef path
	chefDir := filepath.Join(dest, "el", "9", "x86_64", "chef", "18.4.12")
	if _, err := os.Stat(chefDir); err != nil {
		t.Errorf("expected chef directory %s to exist: %v", chefDir, err)
	}

	// Verify inspec path
	inspecDir := filepath.Join(dest, "el", "9", "x86_64", "inspec", "6.8.24")
	if _, err := os.Stat(inspecDir); err != nil {
		t.Errorf("expected inspec directory %s to exist: %v", inspecDir, err)
	}
}

func TestDownload_PlatformFirstLayout_OldLayoutNotCreated(t *testing.T) {
	body := "fake package content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	pkg := testPackage(server.URL+"/chef.rpm", checksum, "chef", "18.4.12", "el", "9", "x86_64")
	_, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Old layout should NOT exist
	oldDir := filepath.Join(dest, "chef", "18.4.12", "el", "9", "x86_64")
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("old layout directory should not exist: %s", oldDir)
	}
}

func TestDownload_GenericProduct_Layout(t *testing.T) {
	body := "fake automate content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	pkg := testPackage(server.URL+"/automate.tar.gz", checksum, "automate", "latest", "linux", "generic", "amd64")
	results, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err != nil {
		t.Errorf("unexpected download error: %v", results[0].Err)
	}

	// Verify platform-first path with generic platform_version
	expectedDir := filepath.Join(dest, "linux", "generic", "amd64", "automate", "latest")
	if _, err := os.Stat(expectedDir); err != nil {
		t.Errorf("expected directory %s to exist: %v", expectedDir, err)
	}
}

// ---------- Basic download operations ----------

func TestDownload_SHA256Mismatch(t *testing.T) {
	body := "fake package content"
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	pkg := testPackage(server.URL+"/chef.rpm", "wrong_checksum", "chef", "18.4.12", "el", "9", "x86_64")
	results, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err == nil {
		t.Error("expected SHA256 mismatch error")
	}
	if !strings.Contains(results[0].Err.Error(), "SHA256 mismatch") {
		t.Errorf("expected SHA256 mismatch error, got: %v", results[0].Err)
	}
}

func TestDownload_EmptySHA256_Accepted(t *testing.T) {
	body := "fake generic product content"
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	// Empty SHA256 should be accepted without verification error
	pkg := testPackage(server.URL+"/automate.tar.gz", "", "automate", "latest", "linux", "generic", "amd64")
	results, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err != nil {
		t.Errorf("empty SHA256 should not cause download failure: %v", results[0].Err)
	}

	// Sidecar should still be written with computed hash
	shaPath := results[0].Path + ".sha256"
	shaContent, err := os.ReadFile(shaPath)
	if err != nil {
		t.Fatalf("reading sha256 sidecar: %v", err)
	}
	computed := sha256sum(body)
	if !strings.Contains(string(shaContent), computed) {
		t.Errorf("sidecar should contain computed hash %s, got %s", computed, string(shaContent))
	}
}

func TestDownload_SkipExisting(t *testing.T) {
	body := "fake package content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1), WithSkipExisting(true))

	pkg := testPackage(server.URL+"/chef.rpm", checksum, "chef", "18.4.12", "el", "9", "x86_64")

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
	d := New(dest, WithConcurrency(1), WithSkipExisting(false))

	pkg := testPackage(server.URL+"/chef.rpm", checksum, "chef", "18.4.12", "el", "9", "x86_64")

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
	// Disable dedup so all 4 download (they share the same body/SHA256)
	d := New(dest, WithConcurrency(4), WithDedup(false))

	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/a.rpm", checksum, "chef", "18.4.12", "el", "9", "x86_64"),
		testPackage(server.URL+"/b.rpm", checksum, "chef", "18.4.12", "el", "8", "x86_64"),
		testPackage(server.URL+"/c.deb", checksum, "chef", "18.4.12", "ubuntu", "22.04", "amd64"),
		testPackage(server.URL+"/d.deb", checksum, "chef", "18.4.12", "debian", "12", "amd64"),
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

	// Verify each is in its own platform-first directory
	dirs := []string{
		filepath.Join(dest, "el", "9", "x86_64", "chef", "18.4.12"),
		filepath.Join(dest, "el", "8", "x86_64", "chef", "18.4.12"),
		filepath.Join(dest, "ubuntu", "22.04", "amd64", "chef", "18.4.12"),
		filepath.Join(dest, "debian", "12", "amd64", "chef", "18.4.12"),
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("expected directory %s to exist: %v", dir, err)
		}
	}
}

func TestDownload_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	pkg := testPackage(server.URL+"/missing.rpm", "abc", "chef", "18.4.12", "el", "9", "x86_64")
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
	d := New(dest, WithConcurrency(1))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	pkg := testPackage(server.URL+"/chef.rpm", "abc", "chef", "18.4.12", "el", "9", "x86_64")
	results, err := d.Download(ctx, []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err == nil {
		t.Error("expected context cancelled error")
	}
}

// ---------- Filename resolution ----------

func TestDownload_FilenameFromURL(t *testing.T) {
	body := "fake package content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	pkg := testPackage(server.URL+"/path/to/chef-18.4.12-1.el9.x86_64.rpm", checksum, "chef", "18.4.12", "el", "9", "x86_64")
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
		{`attachment; filename="chef-ice-19.2.12-1.amzn2.x86_64.rpm"`, "chef-ice-19.2.12-1.amzn2.x86_64.rpm"},
		{`attachment; filename=chef-ice-19.2.12-1.amzn2.x86_64.rpm`, "chef-ice-19.2.12-1.amzn2.x86_64.rpm"},
		{`attachment; filename="chef-18.10.17-1.el10.x86_64.rpm"; filename*=UTF-8''chef-18.10.17-1.el10.x86_64.rpm`, "chef-18.10.17-1.el10.x86_64.rpm"},
		{`inline; filename="report.pdf"`, "report.pdf"},
		{"", ""},
		{"attachment", ""},
		{`attachment; filename="."`, ""},
		{`attachment; filename="/"`, ""},
		{`attachment; filename="../../../etc/passwd"`, "passwd"},
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
	body := "fake chef-ice package"
	checksum := sha256sum(body)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="chef-ice-19.2.12-1.amzn2.x86_64.rpm"`)
		w.Write([]byte(body))
	}))
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	pkg := testPackage(server.URL+"/current/chef-ice/download?p=linux&m=x86_64&pm=rpm&v=19.2.12",
		checksum, "chef-ice", "19.2.12", "linux", "x86_64", "rpm")
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
	body := "fake package"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	pkg := testPackage(server.URL+"/files/chef-18.4.12-1.el9.x86_64.rpm",
		checksum, "chef", "18.4.12", "el", "9", "x86_64")
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

// ---------- Phase 2b: SHA256 deduplication ----------

func TestDownload_DedupSkipsIdenticalSHA256(t *testing.T) {
	body := "identical windows package"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	// Dedup is on by default
	d := New(dest, WithConcurrency(1))

	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "10", "x86_64"),
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "11", "x86_64"),
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "2022", "x86_64"),
	}

	results, err := d.Download(context.Background(), packages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Exactly one should actually download, the other two dedup-skipped.
	// Order is non-deterministic with concurrency, so count rather than
	// assert on specific indices.
	downloadCount := 0
	dedupCount := 0
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
		if r.DedupSkipped {
			dedupCount++
		} else {
			downloadCount++
		}
	}
	if downloadCount != 1 {
		t.Errorf("expected 1 actual download, got %d", downloadCount)
	}
	if dedupCount != 2 {
		t.Errorf("expected 2 dedup skips, got %d", dedupCount)
	}

	// Exactly one platform_version directory should exist on disk
	existCount := 0
	for _, pv := range []string{"10", "11", "2022"} {
		dir := filepath.Join(dest, "windows", pv, "x86_64", "inspec", "6.8.24")
		if _, err := os.Stat(dir); err == nil {
			existCount++
		}
	}
	if existCount != 1 {
		t.Errorf("expected exactly 1 platform_version directory on disk, got %d", existCount)
	}
}

func TestDownload_DedupTracksDifferentSHA256Separately(t *testing.T) {
	bodyA := "package A content"
	bodyB := "package B content"
	checksumA := sha256sum(bodyA)
	checksumB := sha256sum(bodyB)

	mux := http.NewServeMux()
	mux.HandleFunc("/a.rpm", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(bodyA)) })
	mux.HandleFunc("/b.rpm", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(bodyB)) })
	server := httptest.NewServer(mux)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/a.rpm", checksumA, "chef", "18.4.12", "el", "9", "x86_64"),
		testPackage(server.URL+"/b.rpm", checksumB, "inspec", "6.8.24", "el", "9", "x86_64"),
	}

	results, err := d.Download(context.Background(), packages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both should download normally — different SHA256
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result %d: unexpected error: %v", i, r.Err)
		}
		if r.DedupSkipped {
			t.Errorf("result %d: should not be dedup-skipped (different SHA256)", i)
		}
	}
}

func TestDownload_DedupDisabledWithNoDedupOption(t *testing.T) {
	body := "identical content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1), WithDedup(false))

	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "10", "x86_64"),
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "11", "x86_64"),
	}

	results, err := d.Download(context.Background(), packages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both should download — dedup is disabled
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result %d: unexpected error: %v", i, r.Err)
		}
		if r.DedupSkipped {
			t.Errorf("result %d: should not be dedup-skipped when dedup is disabled", i)
		}
	}

	// Both directories should exist
	for _, pv := range []string{"10", "11"} {
		dir := filepath.Join(dest, "windows", pv, "x86_64", "inspec", "6.8.24")
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("expected directory %s to exist: %v", dir, err)
		}
	}
}

func TestDownload_DedupDisabledForEmptySHA256(t *testing.T) {
	body := "generic product content"
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1)) // dedup on by default

	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/automate1.tar.gz", "", "automate", "latest", "linux", "generic", "amd64"),
		testPackage(server.URL+"/automate2.tar.gz", "", "automate", "latest", "darwin", "generic", "amd64"),
	}

	results, err := d.Download(context.Background(), packages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both should download — empty SHA256 disables dedup for those packages
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result %d: unexpected error: %v", i, r.Err)
		}
		if r.DedupSkipped {
			t.Errorf("result %d: should not be dedup-skipped with empty SHA256", i)
		}
	}
}

func TestDownload_DedupOnlyWithinSameBatch(t *testing.T) {
	body := "identical content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	pkg1 := testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "10", "x86_64")
	pkg2 := testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "11", "x86_64")

	// First batch: download pkg1
	results1, _ := d.Download(context.Background(), []chefapi.FlatPackage{pkg1})
	if results1[0].DedupSkipped {
		t.Error("first batch should not dedup")
	}

	// Second batch: download pkg2 — dedup map is per-batch, so this downloads fresh
	results2, _ := d.Download(context.Background(), []chefapi.FlatPackage{pkg2})
	if results2[0].DedupSkipped {
		t.Error("second batch should not dedup from first batch")
	}
}

func TestDownload_DedupConcurrent(t *testing.T) {
	body := "identical concurrent content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(4))

	// 7 Windows versions, all identical — like inspec in the real API
	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "8", "x86_64"),
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "10", "x86_64"),
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "11", "x86_64"),
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "2012", "x86_64"),
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "2012r2", "x86_64"),
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "2016", "x86_64"),
		testPackage(server.URL+"/inspec.msi", checksum, "inspec", "6.8.24", "windows", "2022", "x86_64"),
	}

	results, err := d.Download(context.Background(), packages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	downloadCount := 0
	dedupCount := 0
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
		if r.DedupSkipped {
			dedupCount++
		} else {
			downloadCount++
		}
	}

	// Exactly one should actually download, the rest deduped
	if downloadCount != 1 {
		t.Errorf("expected exactly 1 actual download, got %d", downloadCount)
	}
	if dedupCount != 6 {
		t.Errorf("expected 6 dedup skips, got %d", dedupCount)
	}
}

func TestDownload_DedupAcrossProducts(t *testing.T) {
	// Different platform_versions with same SHA256 should still dedup
	body := "shared binary content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()
	d := New(dest, WithConcurrency(1))

	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/chef.deb", checksum, "chef", "18.4.12", "ubuntu", "20.04", "amd64"),
		testPackage(server.URL+"/chef.deb", checksum, "chef", "18.4.12", "ubuntu", "22.04", "amd64"),
	}

	results, err := d.Download(context.Background(), packages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Exactly one should download, the other dedup-skipped (order non-deterministic)
	downloadCount := 0
	dedupCount := 0
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
		if r.DedupSkipped {
			dedupCount++
		} else {
			downloadCount++
		}
	}
	if downloadCount != 1 {
		t.Errorf("expected 1 actual download, got %d", downloadCount)
	}
	if dedupCount != 1 {
		t.Errorf("expected 1 dedup skip, got %d", dedupCount)
	}
}

// ---------- Phase 2c: Warnings ----------

func TestDownload_WarnsOnEmptySHA256(t *testing.T) {
	body := "generic product"
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()

	var warnings []string
	var mu sync.Mutex
	d := New(dest, WithConcurrency(1), WithWarningFunc(func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		warnings = append(warnings, msg)
	}))

	pkg := testPackage(server.URL+"/automate.tar.gz", "", "automate", "latest", "linux", "generic", "amd64")
	results, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err != nil {
		t.Errorf("unexpected download error: %v", results[0].Err)
	}

	found := false
	for _, w := range warnings {
		if strings.Contains(w, "automate") && strings.Contains(w, "empty SHA256") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about empty SHA256 for automate, got warnings: %v", warnings)
	}
}

func TestDownload_WarnsOnLiteralLatestVersion(t *testing.T) {
	body := "generic product"
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()

	var warnings []string
	var mu sync.Mutex
	d := New(dest, WithConcurrency(1), WithWarningFunc(func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		warnings = append(warnings, msg)
	}))

	pkg := testPackage(server.URL+"/automate.tar.gz", "", "automate", "latest", "linux", "generic", "amd64")
	results, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err != nil {
		t.Errorf("unexpected download error: %v", results[0].Err)
	}

	found := false
	for _, w := range warnings {
		if strings.Contains(w, "automate") && strings.Contains(w, `"latest"`) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about literal 'latest' version for automate, got warnings: %v", warnings)
	}
}

func TestDownload_NoWarningForNormalPackage(t *testing.T) {
	body := "normal package"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()

	var warnings []string
	var mu sync.Mutex
	d := New(dest, WithConcurrency(1), WithWarningFunc(func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		warnings = append(warnings, msg)
	}))

	pkg := testPackage(server.URL+"/chef.rpm", checksum, "chef", "18.4.12", "el", "9", "x86_64")
	results, err := d.Download(context.Background(), []chefapi.FlatPackage{pkg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Err != nil {
		t.Errorf("unexpected download error: %v", results[0].Err)
	}

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for normal package, got: %v", warnings)
	}
}

func TestDownload_BothWarningsEmitted(t *testing.T) {
	body := "generic product"
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()

	var warnings []string
	var mu sync.Mutex
	d := New(dest, WithConcurrency(1), WithWarningFunc(func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		warnings = append(warnings, msg)
	}))

	// Package with both empty SHA256 and literal "latest"
	pkg := testPackage(server.URL+"/automate.tar.gz", "", "automate", "latest", "linux", "generic", "amd64")
	d.Download(context.Background(), []chefapi.FlatPackage{pkg})

	if len(warnings) < 2 {
		t.Errorf("expected at least 2 warnings (empty SHA256 + latest version), got %d: %v", len(warnings), warnings)
	}
}

func TestDownload_WarningOnlyOncePerProduct(t *testing.T) {
	body := "generic product"
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()

	var warnings []string
	var mu sync.Mutex
	d := New(dest, WithConcurrency(1), WithDedup(false), WithWarningFunc(func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		warnings = append(warnings, msg)
	}))

	// Two packages for same product — warnings should fire once per product, not per package
	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/a.tar.gz", "", "automate", "latest", "linux", "generic", "amd64"),
		testPackage(server.URL+"/b.tar.gz", "", "automate", "latest", "darwin", "generic", "amd64"),
	}
	d.Download(context.Background(), packages)

	sha256Warnings := 0
	latestWarnings := 0
	for _, w := range warnings {
		if strings.Contains(w, "empty SHA256") {
			sha256Warnings++
		}
		if strings.Contains(w, `"latest"`) {
			latestWarnings++
		}
	}

	if sha256Warnings != 1 {
		t.Errorf("expected 1 empty SHA256 warning, got %d: %v", sha256Warnings, warnings)
	}
	if latestWarnings != 1 {
		t.Errorf("expected 1 latest version warning, got %d: %v", latestWarnings, warnings)
	}
}

// ---------- Progress callback ----------

func TestDownload_ProgressFunc(t *testing.T) {
	body := "content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()

	var progressCalls []int
	var mu sync.Mutex
	d := New(dest, WithConcurrency(1), WithProgressFunc(func(index, total int, r DownloadResult) {
		mu.Lock()
		defer mu.Unlock()
		progressCalls = append(progressCalls, index)
	}))

	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/a.rpm", checksum, "chef", "18.4.12", "el", "9", "x86_64"),
		testPackage(server.URL+"/b.rpm", checksum, "inspec", "6.8.24", "el", "9", "x86_64"),
	}

	_, err := d.Download(context.Background(), packages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(progressCalls) != 2 {
		t.Errorf("expected 2 progress calls, got %d", len(progressCalls))
	}
}

// ---------- Edge cases ----------

func TestDownload_EmptyPackageList(t *testing.T) {
	dest := t.TempDir()
	d := New(dest)

	results, err := d.Download(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestDownload_ProductFromPackage(t *testing.T) {
	body := "fake content"
	checksum := sha256sum(body)
	server := testServer(t, body)
	defer server.Close()

	dest := t.TempDir()

	// Different products in same batch — each uses its own Product field
	packages := []chefapi.FlatPackage{
		testPackage(server.URL+"/chef.rpm", checksum, "chef", "18.4.12", "el", "9", "x86_64"),
		testPackage(server.URL+"/inspec.rpm", checksum, "inspec", "6.8.24", "el", "9", "x86_64"),
		testPackage(server.URL+"/chef-server.rpm", checksum, "chef-server", "15.10.91", "el", "9", "x86_64"),
	}

	// Dedup off so all download (they share body)
	d := New(dest, WithConcurrency(1), WithDedup(false))
	results, err := d.Download(context.Background(), packages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error for %s: %v", r.Package.Product, r.Err)
		}
	}

	// Each product should have its own directory
	expectedDirs := []string{
		filepath.Join(dest, "el", "9", "x86_64", "chef", "18.4.12"),
		filepath.Join(dest, "el", "9", "x86_64", "inspec", "6.8.24"),
		filepath.Join(dest, "el", "9", "x86_64", "chef-server", "15.10.91"),
	}
	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("expected directory %s to exist: %v", dir, err)
		}
	}
}

// Suppress unused import warnings
var _ = fmt.Sprintf
