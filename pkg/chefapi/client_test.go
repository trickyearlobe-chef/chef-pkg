package chefapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func sampleResponse() PackagesResponse {
	return PackagesResponse{
		"ubuntu": {
			"22.04": {
				"x86_64": PackageDetail{
					SHA1:    "abc123",
					SHA256:  "def456",
					URL:     "https://example.com/chef.deb",
					Version: "19.1.158",
				},
				"aarch64": PackageDetail{
					SHA1:    "aaa111",
					SHA256:  "bbb222",
					URL:     "https://example.com/chef-arm.deb",
					Version: "19.1.158",
				},
			},
		},
		"el": {
			"9": {
				"x86_64": PackageDetail{
					SHA1:    "ccc333",
					SHA256:  "ddd444",
					URL:     "https://example.com/chef.rpm",
					Version: "19.1.158",
				},
			},
			"8": {
				"x86_64": PackageDetail{
					SHA1:    "eee555",
					SHA256:  "fff666",
					URL:     "https://example.com/chef-el8.rpm",
					Version: "19.1.158",
				},
			},
		},
	}
}

func TestFlatten_SortOrder(t *testing.T) {
	resp := sampleResponse()
	flat := resp.Flatten()

	if len(flat) != 4 {
		t.Fatalf("expected 4 packages, got %d", len(flat))
	}

	// Should be sorted: el/8/x86_64, el/9/x86_64, ubuntu/22.04/aarch64, ubuntu/22.04/x86_64
	expected := []struct {
		platform, version, arch string
	}{
		{"el", "8", "x86_64"},
		{"el", "9", "x86_64"},
		{"ubuntu", "22.04", "aarch64"},
		{"ubuntu", "22.04", "x86_64"},
	}

	for i, exp := range expected {
		if flat[i].Platform != exp.platform || flat[i].PlatformVersion != exp.version || flat[i].Architecture != exp.arch {
			t.Errorf("index %d: expected %s/%s/%s, got %s/%s/%s",
				i, exp.platform, exp.version, exp.arch,
				flat[i].Platform, flat[i].PlatformVersion, flat[i].Architecture)
		}
	}
}

func TestFlatten_PreservesDetail(t *testing.T) {
	resp := sampleResponse()
	flat := resp.Flatten()

	// Find the el/9/x86_64 entry
	var found *FlatPackage
	for i := range flat {
		if flat[i].Platform == "el" && flat[i].PlatformVersion == "9" {
			found = &flat[i]
			break
		}
	}
	if found == nil {
		t.Fatal("did not find el/9/x86_64 in flattened results")
	}
	if found.SHA256 != "ddd444" {
		t.Errorf("expected SHA256 ddd444, got %s", found.SHA256)
	}
	if found.URL != "https://example.com/chef.rpm" {
		t.Errorf("expected URL https://example.com/chef.rpm, got %s", found.URL)
	}
}

func TestFlatten_Empty(t *testing.T) {
	resp := PackagesResponse{}
	flat := resp.Flatten()
	if len(flat) != 0 {
		t.Errorf("expected 0 packages, got %d", len(flat))
	}
}

func TestFetchPackages_Success(t *testing.T) {
	sample := sampleResponse()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/current/chef-ice/packages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("v") != "19.1.158" {
			t.Errorf("unexpected version: %s", r.URL.Query().Get("v"))
		}
		if r.URL.Query().Get("license_id") != "test-license" {
			t.Errorf("unexpected license_id: %s", r.URL.Query().Get("license_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sample)
	}))
	defer server.Close()

	client := NewClient("test-license", WithBaseURL(server.URL))
	resp, err := client.FetchPackages(context.Background(), "current", "chef-ice", "19.1.158")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	flat := resp.Flatten()
	if len(flat) != 4 {
		t.Errorf("expected 4 packages, got %d", len(flat))
	}
}

func TestFetchPackages_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Missing license_id query param"))
	}))
	defer server.Close()

	client := NewClient("bad-license", WithBaseURL(server.URL))
	_, err := client.FetchPackages(context.Background(), "current", "chef-ice", "19.1.158")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("expected status 403, got %d", apiErr.StatusCode)
	}
}

func TestFetchPackages_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := NewClient("test-license", WithBaseURL(server.URL))
	_, err := client.FetchPackages(context.Background(), "current", "chef-ice", "19.1.158")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should not be an APIError, should be a JSON decode error
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Error("expected JSON decode error, not APIError")
	}
}

func TestAPIError_Error(t *testing.T) {
	e := &APIError{StatusCode: 403, Body: "forbidden"}
	expected := "chefapi: HTTP 403: forbidden"
	if e.Error() != expected {
		t.Errorf("expected %q, got %q", expected, e.Error())
	}
}
