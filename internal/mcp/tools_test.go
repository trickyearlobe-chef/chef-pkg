package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

// newTestClient creates a chefapi.Client pointing at the given httptest server.
func newTestClient(url string) *chefapi.Client {
	return chefapi.NewClient("test-license", chefapi.WithBaseURL(url))
}

func TestRawGetHandler(t *testing.T) {
	tests := []struct {
		name           string
		input          RawGetInput
		serverHandler  http.HandlerFunc
		wantStatusCode int
		wantPath       string
		wantBodyType   string // "json_array", "json_object", "string", "error_string"
		wantErr        bool
		wantErrContain string
	}{
		{
			name:  "successful JSON array response",
			input: RawGetInput{Path: "/products"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("license_id") == "" {
					http.Error(w, "missing license_id", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]string{"chef", "inspec", "automate"})
			},
			wantStatusCode: 200,
			wantPath:       "/products",
			wantBodyType:   "json_array",
		},
		{
			name:  "successful JSON object response",
			input: RawGetInput{Path: "/stable/chef/packages", Params: map[string]string{"v": "18.4.12"}},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("v") != "18.4.12" {
					http.Error(w, "missing version param", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{
					"ubuntu": map[string]any{
						"22.04": map[string]any{
							"x86_64": map[string]any{
								"sha256":  "abc123",
								"version": "18.4.12",
							},
						},
					},
				})
			},
			wantStatusCode: 200,
			wantPath:       "/stable/chef/packages",
			wantBodyType:   "json_object",
		},
		{
			name:  "params are forwarded as query parameters",
			input: RawGetInput{Path: "/stable/chef/packages", Params: map[string]string{"p": "ubuntu", "m": "x86_64"}},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("p") != "ubuntu" || r.URL.Query().Get("m") != "x86_64" {
					http.Error(w, "missing expected params", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			},
			wantStatusCode: 200,
			wantPath:       "/stable/chef/packages",
			wantBodyType:   "json_object",
		},
		{
			name:  "API error returns output with error status code",
			input: RawGetInput{Path: "/stable/nonexistent/packages"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"product not found"}`))
			},
			wantStatusCode: 404,
			wantPath:       "/stable/nonexistent/packages",
			wantBodyType:   "error_string",
		},
		{
			name:  "API 403 forbidden",
			input: RawGetInput{Path: "/products"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"message":"invalid license"}`))
			},
			wantStatusCode: 403,
			wantPath:       "/products",
			wantBodyType:   "error_string",
		},
		{
			name:           "empty path returns error",
			input:          RawGetInput{Path: ""},
			serverHandler:  func(w http.ResponseWriter, r *http.Request) {},
			wantErr:        true,
			wantErrContain: "path is required",
		},
		{
			name:  "non-JSON response returns raw string",
			input: RawGetInput{Path: "/health"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte("OK"))
			},
			wantStatusCode: 200,
			wantPath:       "/health",
			wantBodyType:   "string",
		},
		{
			name:  "nil params works",
			input: RawGetInput{Path: "/products"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]string{"chef"})
			},
			wantStatusCode: 200,
			wantPath:       "/products",
			wantBodyType:   "json_array",
		},
		{
			name:  "path without leading slash is handled",
			input: RawGetInput{Path: "products"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]string{"chef"})
			},
			wantStatusCode: 200,
			wantPath:       "products",
			wantBodyType:   "json_array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.serverHandler)
			defer srv.Close()

			client := chefapi.NewClient("test-license",
				chefapi.WithBaseURL(srv.URL),
			)

			handler := rawGetHandler(client)

			_, output, err := handler(context.Background(), nil, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantErrContain != "" {
					if got := err.Error(); !contains(got, tt.wantErrContain) {
						t.Errorf("error %q does not contain %q", got, tt.wantErrContain)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if output.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", output.Path, tt.wantPath)
			}

			if output.StatusCode != tt.wantStatusCode {
				t.Errorf("status_code = %d, want %d", output.StatusCode, tt.wantStatusCode)
			}

			switch tt.wantBodyType {
			case "json_array":
				if _, ok := output.Body.([]any); !ok {
					t.Errorf("body type = %T, want []any", output.Body)
				}
			case "json_object":
				if _, ok := output.Body.(map[string]any); !ok {
					t.Errorf("body type = %T, want map[string]any", output.Body)
				}
			case "string":
				if _, ok := output.Body.(string); !ok {
					t.Errorf("body type = %T, want string", output.Body)
				}
			case "error_string":
				// API errors return the body as a string
				if output.Body == nil {
					t.Errorf("body is nil, want non-nil for error response")
				}
			}
		})
	}
}

func TestMapToURLValues(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		want  int // number of keys, or -1 for nil
	}{
		{name: "nil map", input: nil, want: -1},
		{name: "empty map", input: map[string]string{}, want: -1},
		{name: "one key", input: map[string]string{"v": "18"}, want: 1},
		{name: "multiple keys", input: map[string]string{"v": "18", "p": "ubuntu"}, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapToURLValues(tt.input)
			if tt.want == -1 {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			if len(got) != tt.want {
				t.Errorf("len = %d, want %d", len(got), tt.want)
			}
		})
	}
}

// contains checks if s contains substr (avoids importing strings in test).
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestListProductsHandler(t *testing.T) {
	tests := []struct {
		name           string
		input          ListProductsInput
		serverHandler  http.HandlerFunc
		wantCount      int
		wantCurrents   []string // expected current product names
		wantEOLs       []string // expected eol product names
		wantErr        bool
		wantErrContain string
	}{
		{
			name:  "current products only (default)",
			input: ListProductsInput{IncludeEOL: false},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("eol") == "true" {
					t.Error("should not request eol products when include_eol is false")
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]string{"chef", "inspec", "automate"})
			},
			wantCount:    3,
			wantCurrents: []string{"chef", "inspec", "automate"},
		},
		{
			name:  "include eol products",
			input: ListProductsInput{IncludeEOL: true},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				eol := r.URL.Query().Get("eol")
				w.Header().Set("Content-Type", "application/json")
				if eol == "true" {
					// EOL call returns current + eol products
					json.NewEncoder(w).Encode([]string{"chef", "inspec", "automate", "chefdk", "analytics"})
				} else {
					// Current-only call
					json.NewEncoder(w).Encode([]string{"chef", "inspec", "automate"})
				}
			},
			wantCount:    5,
			wantCurrents: []string{"chef", "inspec", "automate"},
			wantEOLs:     []string{"chefdk", "analytics"},
		},
		{
			name:  "empty product list",
			input: ListProductsInput{},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]string{})
			},
			wantCount: 0,
		},
		{
			name:  "API error propagates",
			input: ListProductsInput{},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"message":"invalid license"}`))
			},
			wantErr:        true,
			wantErrContain: "403",
		},
		{
			name:  "eol request with API error on eol call still fails",
			input: ListProductsInput{IncludeEOL: true},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				eol := r.URL.Query().Get("eol")
				if eol == "true" {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"message":"server error"}`))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]string{"chef"})
			},
			wantErr:        true,
			wantErrContain: "500",
		},
		{
			name:  "all eol products are marked eol when they only appear in eol list",
			input: ListProductsInput{IncludeEOL: true},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				eol := r.URL.Query().Get("eol")
				w.Header().Set("Content-Type", "application/json")
				if eol == "true" {
					json.NewEncoder(w).Encode([]string{"chef", "chefdk"})
				} else {
					json.NewEncoder(w).Encode([]string{"chef"})
				}
			},
			wantCount:    2,
			wantCurrents: []string{"chef"},
			wantEOLs:     []string{"chefdk"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.serverHandler)
			defer srv.Close()

			client := newTestClient(srv.URL)
			handler := listProductsHandler(client)

			_, output, err := handler(context.Background(), nil, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantErrContain != "" && !contains(err.Error(), tt.wantErrContain) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrContain)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(output.Products) != tt.wantCount {
				t.Errorf("product count = %d, want %d", len(output.Products), tt.wantCount)
			}

			// Verify current products
			for _, name := range tt.wantCurrents {
				found := false
				for _, p := range output.Products {
					if p.Name == name {
						found = true
						if p.Status != "current" {
							t.Errorf("product %q status = %q, want %q", name, p.Status, "current")
						}
						break
					}
				}
				if !found {
					t.Errorf("expected current product %q not found", name)
				}
			}

			// Verify eol products
			for _, name := range tt.wantEOLs {
				found := false
				for _, p := range output.Products {
					if p.Name == name {
						found = true
						if p.Status != "eol" {
							t.Errorf("product %q status = %q, want %q", name, p.Status, "eol")
						}
						break
					}
				}
				if !found {
					t.Errorf("expected eol product %q not found", name)
				}
			}
		})
	}
}

func TestListVersionsHandler(t *testing.T) {
	tests := []struct {
		name           string
		input          ListVersionsInput
		defaultChannel string
		serverHandler  http.HandlerFunc
		wantProduct    string
		wantChannel    string
		wantVersions   []string
		wantErr        bool
		wantErrContain string
	}{
		{
			name:           "defaults to chef on stable",
			input:          ListVersionsInput{},
			defaultChannel: "stable",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/stable/chef/versions/all" {
					t.Errorf("unexpected path %q", r.URL.Path)
					http.Error(w, "wrong path", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]string{"17.0.0", "18.0.0", "18.4.12"})
			},
			wantProduct:  "chef",
			wantChannel:  "stable",
			wantVersions: []string{"17.0.0", "18.0.0", "18.4.12"},
		},
		{
			name:           "custom product and channel",
			input:          ListVersionsInput{Product: "inspec", Channel: "current"},
			defaultChannel: "stable",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/current/inspec/versions/all" {
					t.Errorf("unexpected path %q, want /current/inspec/versions/all", r.URL.Path)
					http.Error(w, "wrong path", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]string{"5.0.0", "6.0.0"})
			},
			wantProduct:  "inspec",
			wantChannel:  "current",
			wantVersions: []string{"5.0.0", "6.0.0"},
		},
		{
			name:           "uses default channel from config",
			input:          ListVersionsInput{Product: "automate"},
			defaultChannel: "current",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/current/automate/versions/all" {
					t.Errorf("unexpected path %q", r.URL.Path)
					http.Error(w, "wrong path", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]string{"4.0.0"})
			},
			wantProduct:  "automate",
			wantChannel:  "current",
			wantVersions: []string{"4.0.0"},
		},
		{
			name:           "empty version list",
			input:          ListVersionsInput{},
			defaultChannel: "stable",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]string{})
			},
			wantProduct:  "chef",
			wantChannel:  "stable",
			wantVersions: []string{},
		},
		{
			name:           "API error propagates",
			input:          ListVersionsInput{Product: "nonexistent"},
			defaultChannel: "stable",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"product not found"}`))
			},
			wantErr:        true,
			wantErrContain: "404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.serverHandler)
			defer srv.Close()

			client := newTestClient(srv.URL)
			handler := listVersionsHandler(client, tt.defaultChannel)

			_, output, err := handler(context.Background(), nil, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantErrContain != "" && !contains(err.Error(), tt.wantErrContain) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrContain)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if output.Product != tt.wantProduct {
				t.Errorf("product = %q, want %q", output.Product, tt.wantProduct)
			}
			if output.Channel != tt.wantChannel {
				t.Errorf("channel = %q, want %q", output.Channel, tt.wantChannel)
			}

			if len(output.Versions) != len(tt.wantVersions) {
				t.Fatalf("versions count = %d, want %d", len(output.Versions), len(tt.wantVersions))
			}
			for i, v := range tt.wantVersions {
				if output.Versions[i] != v {
					t.Errorf("versions[%d] = %q, want %q", i, output.Versions[i], v)
				}
			}
		})
	}
}

func TestListPackagesHandler(t *testing.T) {
	// Shared mock response for package endpoints
	samplePackages := map[string]map[string]map[string]map[string]any{
		"ubuntu": {
			"22.04": {
				"x86_64": {
					"sha1":    "abc",
					"sha256":  "def123",
					"url":     "https://example.com/chef.deb",
					"version": "18.4.12",
				},
				"aarch64": {
					"sha1":    "ghi",
					"sha256":  "jkl456",
					"url":     "https://example.com/chef-arm.deb",
					"version": "18.4.12",
				},
			},
			"20.04": {
				"x86_64": {
					"sha1":    "mno",
					"sha256":  "pqr789",
					"url":     "https://example.com/chef-2004.deb",
					"version": "18.4.12",
				},
			},
		},
		"el": {
			"9": {
				"x86_64": {
					"sha1":    "stu",
					"sha256":  "vwx012",
					"url":     "https://example.com/chef.rpm",
					"version": "18.4.12",
				},
			},
		},
		"windows": {
			"2019": {
				"x86_64": {
					"sha1":    "yza",
					"sha256":  "bcd345",
					"url":     "https://example.com/chef.msi",
					"version": "18.4.12",
				},
			},
		},
	}

	packageHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(samplePackages)
	}

	tests := []struct {
		name           string
		input          ListPackagesInput
		defaultChannel string
		serverHandler  http.HandlerFunc
		wantProduct    string
		wantVersion    string
		wantChannel    string
		wantCount      int
		wantPlatforms  []string // platforms that should appear in results
		wantNoPlatform []string // platforms that should NOT appear
		wantErr        bool
		wantErrContain string
	}{
		{
			name:           "defaults to latest chef on stable",
			input:          ListPackagesInput{},
			defaultChannel: "stable",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/stable/chef/packages" {
					t.Errorf("unexpected path %q", r.URL.Path)
				}
				if r.URL.Query().Get("v") != "latest" {
					t.Errorf("unexpected version param %q, want latest", r.URL.Query().Get("v"))
				}
				packageHandler(w, r)
			},
			wantProduct: "chef",
			wantVersion: "latest",
			wantChannel: "stable",
			wantCount:   5, // 3 ubuntu + 1 el + 1 windows
		},
		{
			name:           "custom product version and channel",
			input:          ListPackagesInput{Product: "inspec", Version: "5.0.0", Channel: "current"},
			defaultChannel: "stable",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/current/inspec/packages" {
					t.Errorf("unexpected path %q", r.URL.Path)
				}
				if r.URL.Query().Get("v") != "5.0.0" {
					t.Errorf("unexpected version %q", r.URL.Query().Get("v"))
				}
				packageHandler(w, r)
			},
			wantProduct: "inspec",
			wantVersion: "5.0.0",
			wantChannel: "current",
			wantCount:   5,
		},
		{
			name:           "filter by platform",
			input:          ListPackagesInput{Platform: "ubuntu"},
			defaultChannel: "stable",
			serverHandler:  packageHandler,
			wantProduct:    "chef",
			wantVersion:    "latest",
			wantChannel:    "stable",
			wantCount:      3, // ubuntu 22.04 x86_64, ubuntu 22.04 aarch64, ubuntu 20.04 x86_64
			wantPlatforms:  []string{"ubuntu"},
			wantNoPlatform: []string{"el", "windows"},
		},
		{
			name:           "filter by platform case insensitive",
			input:          ListPackagesInput{Platform: "UBUNTU"},
			defaultChannel: "stable",
			serverHandler:  packageHandler,
			wantProduct:    "chef",
			wantVersion:    "latest",
			wantChannel:    "stable",
			wantCount:      3,
			wantPlatforms:  []string{"ubuntu"},
		},
		{
			name:           "filter by arch",
			input:          ListPackagesInput{Arch: "aarch64"},
			defaultChannel: "stable",
			serverHandler:  packageHandler,
			wantProduct:    "chef",
			wantVersion:    "latest",
			wantChannel:    "stable",
			wantCount:      1, // only ubuntu 22.04 aarch64
		},
		{
			name:           "filter by platform and arch combined",
			input:          ListPackagesInput{Platform: "ubuntu", Arch: "x86_64"},
			defaultChannel: "stable",
			serverHandler:  packageHandler,
			wantProduct:    "chef",
			wantVersion:    "latest",
			wantChannel:    "stable",
			wantCount:      2, // ubuntu 22.04 x86_64, ubuntu 20.04 x86_64
		},
		{
			name:           "filter matches nothing",
			input:          ListPackagesInput{Platform: "solaris"},
			defaultChannel: "stable",
			serverHandler:  packageHandler,
			wantProduct:    "chef",
			wantVersion:    "latest",
			wantChannel:    "stable",
			wantCount:      0,
		},
		{
			name:           "URL is not included in output",
			input:          ListPackagesInput{Platform: "el"},
			defaultChannel: "stable",
			serverHandler:  packageHandler,
			wantProduct:    "chef",
			wantVersion:    "latest",
			wantChannel:    "stable",
			wantCount:      1,
		},
		{
			name:           "API error propagates",
			input:          ListPackagesInput{Product: "nonexistent"},
			defaultChannel: "stable",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"product not found"}`))
			},
			wantErr:        true,
			wantErrContain: "404",
		},
		{
			name:           "partial platform substring match",
			input:          ListPackagesInput{Platform: "win"},
			defaultChannel: "stable",
			serverHandler:  packageHandler,
			wantProduct:    "chef",
			wantVersion:    "latest",
			wantChannel:    "stable",
			wantCount:      1,
			wantPlatforms:  []string{"windows"},
		},
		{
			name:           "arch substring match",
			input:          ListPackagesInput{Arch: "x86"},
			defaultChannel: "stable",
			serverHandler:  packageHandler,
			wantProduct:    "chef",
			wantVersion:    "latest",
			wantChannel:    "stable",
			wantCount:      4, // all x86_64 entries
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.serverHandler)
			defer srv.Close()

			client := newTestClient(srv.URL)
			handler := listPackagesHandler(client, tt.defaultChannel)

			_, output, err := handler(context.Background(), nil, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantErrContain != "" && !contains(err.Error(), tt.wantErrContain) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrContain)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if output.Product != tt.wantProduct {
				t.Errorf("product = %q, want %q", output.Product, tt.wantProduct)
			}
			if output.Version != tt.wantVersion {
				t.Errorf("version = %q, want %q", output.Version, tt.wantVersion)
			}
			if output.Channel != tt.wantChannel {
				t.Errorf("channel = %q, want %q", output.Channel, tt.wantChannel)
			}

			if len(output.Packages) != tt.wantCount {
				t.Fatalf("package count = %d, want %d; packages: %+v", len(output.Packages), tt.wantCount, output.Packages)
			}

			// Verify expected platforms appear
			for _, plat := range tt.wantPlatforms {
				found := false
				for _, pkg := range output.Packages {
					if pkg.Platform == plat {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected platform %q not found in results", plat)
				}
			}

			// Verify excluded platforms do NOT appear
			for _, plat := range tt.wantNoPlatform {
				for _, pkg := range output.Packages {
					if pkg.Platform == plat {
						t.Errorf("platform %q should not appear in filtered results", plat)
						break
					}
				}
			}

			// Verify URL is never present in PackageInfo output
			for _, pkg := range output.Packages {
				raw, _ := json.Marshal(pkg)
				if contains(string(raw), "url") {
					t.Errorf("PackageInfo should not contain url field, got: %s", string(raw))
				}
			}

			// Verify SHA256 is populated when packages exist
			for _, pkg := range output.Packages {
				if pkg.SHA256 == "" {
					t.Errorf("package %s/%s/%s has empty SHA256", pkg.Platform, pkg.PlatformVersion, pkg.Architecture)
				}
			}
		})
	}
}
