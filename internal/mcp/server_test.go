package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

// setupTestServer creates a mock Chef API HTTP server and returns an MCP
// server connected to it, along with a connected MCP client session.
func setupTestServer(t *testing.T, handler http.Handler) (*sdkmcp.Server, *sdkmcp.ClientSession) {
	t.Helper()

	apiServer := httptest.NewServer(handler)
	t.Cleanup(apiServer.Close)

	client := chefapi.NewClient("test-license",
		chefapi.WithBaseURL(apiServer.URL),
	)

	mcpServer := NewServer(ServerConfig{
		Version: "test",
		Channel: "stable",
		Client:  client,
	})

	ctx := context.Background()
	t1, t2 := sdkmcp.NewInMemoryTransports()

	if _, err := mcpServer.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	mcpClient := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "0.0.1"},
		nil,
	)

	cs, err := mcpClient.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })

	return mcpServer, cs
}

func TestServerListsTools(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})

	_, cs := setupTestServer(t, mux)

	result, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(result.Tools) == 0 {
		t.Fatal("expected at least one tool, got zero")
	}

	expectedTools := []string{"raw_get", "list_products", "list_versions", "list_packages"}
	names := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		names[i] = tool.Name
	}

	for _, want := range expectedTools {
		found := false
		for _, name := range names {
			if name == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tool %q not found in tools: %v", want, names)
		}
	}

	if len(result.Tools) != len(expectedTools) {
		t.Errorf("tool count = %d, want %d; tools: %v", len(result.Tools), len(expectedTools), names)
	}
}

func TestServerRawGetTool(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("license_id") == "" {
			http.Error(w, "missing license_id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{"chef", "inspec", "automate"})
	})

	_, cs := setupTestServer(t, mux)

	result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "raw_get",
		Arguments: map[string]any{"path": "/products"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %+v", result.Content)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content, got empty")
	}

	// The structured output should be in StructuredContent
	if result.StructuredContent != nil {
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatalf("marshal structured content: %v", err)
		}
		var output RawGetOutput
		if err := json.Unmarshal(raw, &output); err != nil {
			t.Fatalf("unmarshal structured content: %v", err)
		}
		if output.StatusCode != 200 {
			t.Errorf("status_code = %d, want 200", output.StatusCode)
		}
		if output.Path != "/products" {
			t.Errorf("path = %q, want /products", output.Path)
		}
	}
}

func TestServerRawGetWithParams(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stable/chef/packages", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("v") != "18.4.12" {
			http.Error(w, "expected v=18.4.12", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ubuntu": map[string]any{}})
	})

	_, cs := setupTestServer(t, mux)

	result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "raw_get",
		Arguments: map[string]any{
			"path":   "/stable/chef/packages",
			"params": map[string]any{"v": "18.4.12"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %+v", result.Content)
	}
}

func TestServerRawGetAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	})

	_, cs := setupTestServer(t, mux)

	result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "raw_get",
		Arguments: map[string]any{"path": "/nonexistent"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	// API errors with status codes are returned as successful tool output
	// (not isError) because the tool did its job — it made the request and
	// reported the response. The status_code field tells the LLM what happened.
	if result.IsError {
		t.Logf("tool returned error (acceptable): %+v", result.Content)
		return
	}

	if result.StructuredContent != nil {
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatalf("marshal structured content: %v", err)
		}
		var output RawGetOutput
		if err := json.Unmarshal(raw, &output); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if output.StatusCode != 404 {
			t.Errorf("status_code = %d, want 404", output.StatusCode)
		}
	}
}

func TestServerRawGetEmptyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	_, cs := setupTestServer(t, mux)

	result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "raw_get",
		Arguments: map[string]any{"path": ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if !result.IsError {
		t.Error("expected tool error for empty path, got success")
	}
}

func TestServerIdentity(t *testing.T) {
	server := NewServer(ServerConfig{
		Version: "1.2.3",
		Client:  chefapi.NewClient("test"),
	})

	if server == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestServerDefaultVersion(t *testing.T) {
	server := NewServer(ServerConfig{
		Version: "",
		Client:  chefapi.NewClient("test"),
	})

	if server == nil {
		t.Fatal("NewServer returned nil with empty version")
	}
}

func TestServerListProductsTool(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("eol") == "true" {
			json.NewEncoder(w).Encode([]string{"chef", "inspec", "chefdk"})
		} else {
			json.NewEncoder(w).Encode([]string{"chef", "inspec"})
		}
	})

	_, cs := setupTestServer(t, mux)

	t.Run("current only", func(t *testing.T) {
		result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name:      "list_products",
			Arguments: map[string]any{},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if result.IsError {
			t.Fatalf("tool returned error: %+v", result.Content)
		}
		if result.StructuredContent == nil {
			t.Fatal("expected structured content")
		}
		raw, _ := json.Marshal(result.StructuredContent)
		var output ListProductsOutput
		if err := json.Unmarshal(raw, &output); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(output.Products) != 2 {
			t.Errorf("product count = %d, want 2", len(output.Products))
		}
		for _, p := range output.Products {
			if p.Status != "current" {
				t.Errorf("product %q status = %q, want current", p.Name, p.Status)
			}
		}
	})

	t.Run("include eol", func(t *testing.T) {
		result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name:      "list_products",
			Arguments: map[string]any{"include_eol": true},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if result.IsError {
			t.Fatalf("tool returned error: %+v", result.Content)
		}
		raw, _ := json.Marshal(result.StructuredContent)
		var output ListProductsOutput
		if err := json.Unmarshal(raw, &output); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(output.Products) != 3 {
			t.Errorf("product count = %d, want 3", len(output.Products))
		}
		// chefdk should be tagged eol
		for _, p := range output.Products {
			if p.Name == "chefdk" && p.Status != "eol" {
				t.Errorf("chefdk status = %q, want eol", p.Status)
			}
		}
	})
}

func TestServerListVersionsTool(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stable/chef/versions/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{"17.0.0", "18.0.0", "18.4.12"})
	})
	mux.HandleFunc("/current/inspec/versions/all", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{"5.0.0", "6.0.0"})
	})

	_, cs := setupTestServer(t, mux)

	t.Run("defaults to chef stable", func(t *testing.T) {
		result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name:      "list_versions",
			Arguments: map[string]any{},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if result.IsError {
			t.Fatalf("tool returned error: %+v", result.Content)
		}
		raw, _ := json.Marshal(result.StructuredContent)
		var output ListVersionsOutput
		if err := json.Unmarshal(raw, &output); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if output.Product != "chef" {
			t.Errorf("product = %q, want chef", output.Product)
		}
		if output.Channel != "stable" {
			t.Errorf("channel = %q, want stable", output.Channel)
		}
		if len(output.Versions) != 3 {
			t.Errorf("version count = %d, want 3", len(output.Versions))
		}
	})

	t.Run("custom product and channel", func(t *testing.T) {
		result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name: "list_versions",
			Arguments: map[string]any{
				"product": "inspec",
				"channel": "current",
			},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if result.IsError {
			t.Fatalf("tool returned error: %+v", result.Content)
		}
		raw, _ := json.Marshal(result.StructuredContent)
		var output ListVersionsOutput
		if err := json.Unmarshal(raw, &output); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if output.Product != "inspec" {
			t.Errorf("product = %q, want inspec", output.Product)
		}
		if output.Channel != "current" {
			t.Errorf("channel = %q, want current", output.Channel)
		}
		if len(output.Versions) != 2 {
			t.Errorf("version count = %d, want 2", len(output.Versions))
		}
	})
}

func TestServerListPackagesTool(t *testing.T) {
	samplePackages := map[string]map[string]map[string]map[string]any{
		"ubuntu": {
			"22.04": {
				"x86_64": {
					"sha1": "abc", "sha256": "def123",
					"url": "https://example.com/chef.deb", "version": "18.4.12",
				},
				"aarch64": {
					"sha1": "ghi", "sha256": "jkl456",
					"url": "https://example.com/chef-arm.deb", "version": "18.4.12",
				},
			},
		},
		"el": {
			"9": {
				"x86_64": {
					"sha1": "stu", "sha256": "vwx012",
					"url": "https://example.com/chef.rpm", "version": "18.4.12",
				},
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/stable/chef/packages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(samplePackages)
	})

	_, cs := setupTestServer(t, mux)

	t.Run("default returns all packages", func(t *testing.T) {
		result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name:      "list_packages",
			Arguments: map[string]any{},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if result.IsError {
			t.Fatalf("tool returned error: %+v", result.Content)
		}
		raw, _ := json.Marshal(result.StructuredContent)
		var output ListPackagesOutput
		if err := json.Unmarshal(raw, &output); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if output.Product != "chef" {
			t.Errorf("product = %q, want chef", output.Product)
		}
		if output.Channel != "stable" {
			t.Errorf("channel = %q, want stable", output.Channel)
		}
		if output.Version != "latest" {
			t.Errorf("version = %q, want latest", output.Version)
		}
		if len(output.Packages) != 3 {
			t.Errorf("package count = %d, want 3", len(output.Packages))
		}
		// Verify URL is not in the serialized output
		if strings.Contains(string(raw), "example.com") {
			t.Error("output should not contain URL (it embeds license_id)")
		}
	})

	t.Run("filter by platform", func(t *testing.T) {
		result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name: "list_packages",
			Arguments: map[string]any{
				"platform": "ubuntu",
			},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if result.IsError {
			t.Fatalf("tool returned error: %+v", result.Content)
		}
		raw, _ := json.Marshal(result.StructuredContent)
		var output ListPackagesOutput
		if err := json.Unmarshal(raw, &output); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(output.Packages) != 2 {
			t.Errorf("package count = %d, want 2 (ubuntu only)", len(output.Packages))
		}
		for _, pkg := range output.Packages {
			if pkg.Platform != "ubuntu" {
				t.Errorf("unexpected platform %q in filtered results", pkg.Platform)
			}
		}
	})

	t.Run("filter by arch", func(t *testing.T) {
		result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name: "list_packages",
			Arguments: map[string]any{
				"arch": "aarch64",
			},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if result.IsError {
			t.Fatalf("tool returned error: %+v", result.Content)
		}
		raw, _ := json.Marshal(result.StructuredContent)
		var output ListPackagesOutput
		if err := json.Unmarshal(raw, &output); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(output.Packages) != 1 {
			t.Errorf("package count = %d, want 1 (aarch64 only)", len(output.Packages))
		}
	})

	t.Run("SHA256 populated", func(t *testing.T) {
		result, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name: "list_packages",
			Arguments: map[string]any{
				"platform": "el",
			},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		raw, _ := json.Marshal(result.StructuredContent)
		var output ListPackagesOutput
		if err := json.Unmarshal(raw, &output); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(output.Packages) != 1 {
			t.Fatalf("package count = %d, want 1", len(output.Packages))
		}
		if output.Packages[0].SHA256 != "vwx012" {
			t.Errorf("sha256 = %q, want vwx012", output.Packages[0].SHA256)
		}
	})
}
