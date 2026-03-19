package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestListProducts_IncludeObsolete_Table(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/products" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("eol") == "true" {
			json.NewEncoder(w).Encode([]string{"chef", "oldproduct", "inspec"})
		} else {
			json.NewEncoder(w).Encode([]string{"chef", "inspec"})
		}
	}))
	defer server.Close()

	viper.Set("chef.license_id", "test-license")
	viper.Set("chef.base_url", server.URL)
	viper.Set("chef.channel", "stable")
	defer viper.Reset()

	out := captureStdout(t, func() {
		cmd := rootCmd
		cmd.SetArgs([]string{"list", "products", "--include-eol"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "chef") {
		t.Fatalf("expected output to contain 'chef', got: %s", out)
	}
	if !strings.Contains(out, "oldproduct") {
		t.Fatalf("expected output to contain 'oldproduct', got: %s", out)
	}
	if !strings.Contains(out, "current") {
		t.Fatalf("expected 'current' status for chef, got: %s", out)
	}
	if !strings.Contains(out, "eol") {
		t.Fatalf("expected 'eol' status for oldproduct, got: %s", out)
	}
}

func TestListProducts_IncludeObsolete_JSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/products" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("eol") == "true" {
			json.NewEncoder(w).Encode([]string{"chef", "deadprod"})
		} else {
			json.NewEncoder(w).Encode([]string{"chef"})
		}
	}))
	defer server.Close()

	viper.Set("chef.license_id", "test-license")
	viper.Set("chef.base_url", server.URL)
	viper.Set("chef.channel", "stable")
	defer viper.Reset()

	out := captureStdout(t, func() {
		cmd := rootCmd
		cmd.SetArgs([]string{"list", "products", "--include-eol", "-o", "json"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var results []map[string]string
	if err := json.NewDecoder(strings.NewReader(out)).Decode(&results); err != nil {
		t.Fatalf("decoding JSON: %v\noutput was: %s", err, out)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	statusByProduct := map[string]string{}
	for _, r := range results {
		statusByProduct[r["product"]] = r["status"]
	}
	if statusByProduct["chef"] != "current" {
		t.Errorf("expected chef status=current, got %q", statusByProduct["chef"])
	}
	if statusByProduct["deadprod"] != "eol" {
		t.Errorf("expected deadprod status=eol, got %q", statusByProduct["deadprod"])
	}
}
