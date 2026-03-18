package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input               string
		major, minor, patch int
		ok                  bool
	}{
		{"18.4.12", 18, 4, 12, true},
		{"1.0.0", 1, 0, 0, true},
		{"0.1.2", 0, 1, 2, true},
		{"v18.4.12", 18, 4, 12, true},
		{"18.4", 18, 4, 0, true},
		{"18", 18, 0, 0, true},
		{"18.4.12-rc1", 18, 4, 12, true},
		{"18.4.12+build.1", 18, 4, 12, true},
		{"18.4.12-beta+build", 18, 4, 12, true},
		{"latest", 0, 0, 0, false},
		{"all", 0, 0, 0, false},
		{"", 0, 0, 0, false},
		{"abc", 0, 0, 0, false},
		{"1.2.3.4", 0, 0, 0, false},
		{"1.a.3", 0, 0, 0, false},
		{"1..3", 0, 0, 0, false},
		{".1.2", 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			major, minor, patch, ok := parseSemver(tt.input)
			if ok != tt.ok {
				t.Errorf("parseSemver(%q): ok = %v, want %v", tt.input, ok, tt.ok)
				return
			}
			if !ok {
				return
			}
			if major != tt.major || minor != tt.minor || patch != tt.patch {
				t.Errorf("parseSemver(%q) = (%d, %d, %d), want (%d, %d, %d)",
					tt.input, major, minor, patch, tt.major, tt.minor, tt.patch)
			}
		})
	}
}

func TestSemverLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "2.0.0", true},
		{"2.0.0", "1.0.0", false},
		{"1.0.0", "1.1.0", true},
		{"1.1.0", "1.0.0", false},
		{"1.0.0", "1.0.1", true},
		{"1.0.1", "1.0.0", false},
		{"1.0.0", "1.0.0", false},
		{"18.4.12", "18.5.0", true},
		{"18.5.0", "18.4.12", false},
		{"1.9.0", "1.10.0", true},
		{"1.10.0", "1.9.0", false},
		// Non-semver strings sort before valid semver
		{"abc", "1.0.0", true},
		{"1.0.0", "abc", false},
		// Two non-semver strings sort lexically
		{"abc", "def", true},
		{"def", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := semverLess(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("semverLess(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSortVersionsSemver(t *testing.T) {
	versions := []string{"18.5.0", "18.4.12", "19.0.1", "17.0.0", "18.4.2"}
	sortVersionsSemver(versions)

	expected := []string{"17.0.0", "18.4.2", "18.4.12", "18.5.0", "19.0.1"}
	for i, v := range versions {
		if v != expected[i] {
			t.Errorf("index %d: got %s, want %s", i, v, expected[i])
		}
	}
}

func TestSortVersionsSemver_WithNonSemver(t *testing.T) {
	versions := []string{"18.4.12", "latest", "1.0.0", "abc"}
	sortVersionsSemver(versions)

	// Non-semver should sort before valid semver
	if versions[len(versions)-1] != "18.4.12" {
		t.Errorf("expected 18.4.12 last, got %s", versions[len(versions)-1])
	}
	// The first two should be the non-semver strings
	for _, v := range versions[:2] {
		if _, _, _, ok := parseSemver(v); ok {
			t.Errorf("expected non-semver in first positions, got %s", v)
		}
	}
}

func TestSortVersionsSemver_Empty(t *testing.T) {
	var versions []string
	sortVersionsSemver(versions) // Should not panic
	if len(versions) != 0 {
		t.Errorf("expected empty slice, got %v", versions)
	}
}

func TestSortVersionsSemver_Single(t *testing.T) {
	versions := []string{"1.0.0"}
	sortVersionsSemver(versions)
	if versions[0] != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", versions[0])
	}
}

// testVersionServer creates an httptest server that serves version and package endpoints.
func testVersionServer(t *testing.T, versions []string, packagesByVersion map[string]chefapi.PackagesResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/stable/chef/versions/all":
			json.NewEncoder(w).Encode(versions)
		case r.URL.Path == "/stable/chef/packages":
			v := r.URL.Query().Get("v")
			if pkgs, ok := packagesByVersion[v]; ok {
				json.NewEncoder(w).Encode(pkgs)
			} else {
				json.NewEncoder(w).Encode(chefapi.PackagesResponse{})
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestResolveVersions_Exact(t *testing.T) {
	versions, err := resolveVersions(context.Background(), nil, "stable", "chef", "18.4.12", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 || versions[0] != "18.4.12" {
		t.Errorf("expected [18.4.12], got %v", versions)
	}
}

func TestResolveVersions_MajorOnly(t *testing.T) {
	allVersions := []string{"19.0.0", "18.5.0", "18.4.12", "17.9.9"}
	server := testVersionServer(t, allVersions, nil)
	defer server.Close()

	client := chefapi.NewClient("test-license", chefapi.WithBaseURL(server.URL))

	versions, err := resolveVersions(context.Background(), client, "stable", "chef", "18", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 2 || versions[0] != "18.4.12" || versions[1] != "18.5.0" {
		t.Errorf("expected [18.5.0], got %v", versions)
	}
}

func TestResolveVersions_MajorOnly_WithPlatformFilter(t *testing.T) {
	allVersions := []string{"19.0.0", "18.5.0", "18.4.12"}
	packagesByVersion := map[string]chefapi.PackagesResponse{
		"19.0.0": {
			"el": {
				"9": {
					"x86_64": chefapi.PackageDetail{Version: "19.0.0"},
				},
			},
		},
		"18.5.0": {
			"ubuntu": {
				"22.04": {
					"x86_64": chefapi.PackageDetail{Version: "18.5.0"},
				},
			},
		},
	}
	server := testVersionServer(t, allVersions, packagesByVersion)
	defer server.Close()

	client := chefapi.NewClient("test-license", chefapi.WithBaseURL(server.URL))

	versions, err := resolveVersions(context.Background(), client, "stable", "chef", "18", "ubuntu", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 || versions[0] != "18.5.0" {
		t.Errorf("expected [18.5.0], got %v", versions)
	}
}

func TestResolveVersions_MajorOnly_NoMatch(t *testing.T) {
	allVersions := []string{"19.0.0", "18.5.0"}
	server := testVersionServer(t, allVersions, nil)
	defer server.Close()

	client := chefapi.NewClient("test-license", chefapi.WithBaseURL(server.URL))

	_, err := resolveVersions(context.Background(), client, "stable", "chef", "18", "solaris", "")
	if err == nil {
		t.Fatal("expected error when no version matches major-only filter")
	}
}

func TestResolveVersions_MajorOnly_FallsBackToExactMajor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/stable/chef/versions/all":
			json.NewEncoder(w).Encode([]string{"19.0.0", "18.5.0"})
		case r.URL.Path == "/stable/chef/packages":
			if r.URL.Query().Get("v") == "17" {
				json.NewEncoder(w).Encode(chefapi.PackagesResponse{
					"el": {
						"9": {
							"x86_64": chefapi.PackageDetail{Version: "17.9.9"},
						},
					},
				})
				return
			}
			json.NewEncoder(w).Encode(chefapi.PackagesResponse{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := chefapi.NewClient("test-license", chefapi.WithBaseURL(server.URL))

	versions, err := resolveVersions(context.Background(), client, "stable", "chef", "17", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 || versions[0] != "17" {
		t.Errorf("expected [17], got %v", versions)
	}
}

func TestResolveVersions_ExactInvalid(t *testing.T) {
	_, err := resolveVersions(context.Background(), nil, "stable", "chef", "not-a-version", "", "")
	if err == nil {
		t.Error("expected error for invalid version")
	}
}

func TestResolveVersions_All(t *testing.T) {
	allVersions := []string{"18.5.0", "17.0.0", "18.4.12"}
	server := testVersionServer(t, allVersions, nil)
	defer server.Close()

	client := chefapi.NewClient("test-license", chefapi.WithBaseURL(server.URL))

	versions, err := resolveVersions(context.Background(), client, "stable", "chef", "all", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	// Should be sorted ascending
	expected := []string{"17.0.0", "18.4.12", "18.5.0"}
	for i, v := range versions {
		if v != expected[i] {
			t.Errorf("index %d: got %s, want %s", i, v, expected[i])
		}
	}
}

func TestResolveVersions_Latest_NoFilter(t *testing.T) {
	allVersions := []string{"18.5.0", "17.0.0", "18.4.12"}
	server := testVersionServer(t, allVersions, nil)
	defer server.Close()

	client := chefapi.NewClient("test-license", chefapi.WithBaseURL(server.URL))

	versions, err := resolveVersions(context.Background(), client, "stable", "chef", "latest", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 || versions[0] != "18.5.0" {
		t.Errorf("expected [18.5.0], got %v", versions)
	}
}

func TestResolveVersions_Latest_WithPlatformFilter(t *testing.T) {
	allVersions := []string{"18.5.0", "18.4.12", "17.0.0"}

	// 18.5.0 has only el packages, 18.4.12 has ubuntu and el packages
	packagesByVersion := map[string]chefapi.PackagesResponse{
		"18.5.0": {
			"el": {
				"9": {
					"x86_64": chefapi.PackageDetail{
						URL: "http://example.com/chef-18.5.0.el9.x86_64.rpm", SHA256: "abc", Version: "18.5.0",
					},
				},
			},
		},
		"18.4.12": {
			"ubuntu": {
				"22.04": {
					"x86_64": chefapi.PackageDetail{
						URL: "http://example.com/chef_18.4.12_amd64.deb", SHA256: "def", Version: "18.4.12",
					},
				},
			},
			"el": {
				"9": {
					"x86_64": chefapi.PackageDetail{
						URL: "http://example.com/chef-18.4.12.el9.x86_64.rpm", SHA256: "ghi", Version: "18.4.12",
					},
				},
			},
		},
		"17.0.0": {
			"ubuntu": {
				"22.04": {
					"x86_64": chefapi.PackageDetail{
						URL: "http://example.com/chef_17.0.0_amd64.deb", SHA256: "jkl", Version: "17.0.0",
					},
				},
			},
		},
	}

	server := testVersionServer(t, allVersions, packagesByVersion)
	defer server.Close()

	client := chefapi.NewClient("test-license", chefapi.WithBaseURL(server.URL))

	// Filtering by ubuntu should skip 18.5.0 (no ubuntu packages) and return 18.4.12
	versions, err := resolveVersions(context.Background(), client, "stable", "chef", "latest", "ubuntu", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 || versions[0] != "18.4.12" {
		t.Errorf("expected [18.4.12], got %v", versions)
	}
}

func TestResolveVersions_Latest_WithArchFilter(t *testing.T) {
	allVersions := []string{"19.0.0", "18.5.0"}

	packagesByVersion := map[string]chefapi.PackagesResponse{
		"19.0.0": {
			"el": {
				"9": {
					"x86_64": chefapi.PackageDetail{
						URL: "http://example.com/chef-19.0.0.el9.x86_64.rpm", SHA256: "abc", Version: "19.0.0",
					},
				},
			},
		},
		"18.5.0": {
			"el": {
				"9": {
					"aarch64": chefapi.PackageDetail{
						URL: "http://example.com/chef-18.5.0.el9.aarch64.rpm", SHA256: "def", Version: "18.5.0",
					},
				},
			},
		},
	}

	server := testVersionServer(t, allVersions, packagesByVersion)
	defer server.Close()

	client := chefapi.NewClient("test-license", chefapi.WithBaseURL(server.URL))

	// Filtering by aarch64 should skip 19.0.0 and return 18.5.0
	versions, err := resolveVersions(context.Background(), client, "stable", "chef", "latest", "", "aarch64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 || versions[0] != "18.5.0" {
		t.Errorf("expected [18.5.0], got %v", versions)
	}
}

func TestResolveVersions_Latest_NoneMatch(t *testing.T) {
	allVersions := []string{"18.5.0"}

	packagesByVersion := map[string]chefapi.PackagesResponse{
		"18.5.0": {
			"el": {
				"9": {
					"x86_64": chefapi.PackageDetail{
						URL: "http://example.com/chef-18.5.0.el9.x86_64.rpm", SHA256: "abc", Version: "18.5.0",
					},
				},
			},
		},
	}

	server := testVersionServer(t, allVersions, packagesByVersion)
	defer server.Close()

	client := chefapi.NewClient("test-license", chefapi.WithBaseURL(server.URL))

	// Filtering by solaris should find no match
	_, err := resolveVersions(context.Background(), client, "stable", "chef", "latest", "solaris", "")
	if err == nil {
		t.Error("expected error when no version matches platform filter")
	}
}

func TestResolveVersions_CaseInsensitive(t *testing.T) {
	// "LATEST" and "ALL" should work the same as lowercase
	allVersions := []string{"18.4.12"}
	server := testVersionServer(t, allVersions, nil)
	defer server.Close()

	client := chefapi.NewClient("test-license", chefapi.WithBaseURL(server.URL))

	versions, err := resolveVersions(context.Background(), client, "stable", "chef", "LATEST", "", "")
	if err != nil {
		t.Fatalf("unexpected error for LATEST: %v", err)
	}
	if len(versions) != 1 || versions[0] != "18.4.12" {
		t.Errorf("LATEST: expected [18.4.12], got %v", versions)
	}

	versions, err = resolveVersions(context.Background(), client, "stable", "chef", "ALL", "", "")
	if err != nil {
		t.Fatalf("unexpected error for ALL: %v", err)
	}
	if len(versions) != 1 || versions[0] != "18.4.12" {
		t.Errorf("ALL: expected [18.4.12], got %v", versions)
	}
}

func TestResolveVersions_Latest_CombinedFilter(t *testing.T) {
	allVersions := []string{"19.0.0", "18.5.0", "18.4.12"}

	packagesByVersion := map[string]chefapi.PackagesResponse{
		"19.0.0": {
			"ubuntu": {
				"22.04": {
					"x86_64": chefapi.PackageDetail{
						URL: "http://example.com/chef_19.0.0_amd64.deb", SHA256: "abc", Version: "19.0.0",
					},
				},
			},
		},
		"18.5.0": {
			"ubuntu": {
				"22.04": {
					"aarch64": chefapi.PackageDetail{
						URL: "http://example.com/chef_18.5.0_arm64.deb", SHA256: "def", Version: "18.5.0",
					},
				},
			},
		},
		"18.4.12": {
			"ubuntu": {
				"22.04": {
					"aarch64": chefapi.PackageDetail{
						URL: "http://example.com/chef_18.4.12_arm64.deb", SHA256: "ghi", Version: "18.4.12",
					},
				},
			},
		},
	}

	server := testVersionServer(t, allVersions, packagesByVersion)
	defer server.Close()

	client := chefapi.NewClient("test-license", chefapi.WithBaseURL(server.URL))

	// ubuntu+aarch64: 19.0.0 has ubuntu but only x86_64, so should resolve to 18.5.0
	versions, err := resolveVersions(context.Background(), client, "stable", "chef", "latest", "ubuntu", "aarch64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 || versions[0] != "18.5.0" {
		t.Errorf("expected [18.5.0], got %v", versions)
	}
}
