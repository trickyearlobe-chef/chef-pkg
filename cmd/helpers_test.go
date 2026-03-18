package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	fn()

	_ = w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestOutputTable_DoesNotPrintURL(t *testing.T) {
	packages := []chefapi.FlatPackage{
		{
			Platform:        "el",
			PlatformVersion: "9",
			Architecture:    "x86_64",
			PackageDetail: chefapi.PackageDetail{
				Version: "18.9.4",
				URL:     "https://commercial-acceptance.downloads.chef.co/stable/chef/download?license_id=SECRET&v=18.9.4",
				SHA256:   strings.Repeat("a", 64),
			},
		},
	}

	out := captureStdout(t, func() {
		if err := outputTable(packages); err != nil {
			t.Fatal(err)
		}
	})

	if strings.Contains(out, "license_id=") {
		t.Fatalf("outputTable leaked license_id: %s", out)
	}
	if strings.Contains(out, "downloads.chef.co") {
		t.Fatalf("outputTable leaked URL: %s", out)
	}
}

func TestOutputJSON_RedactsURLField(t *testing.T) {
	packages := []chefapi.FlatPackage{
		{
			Platform:        "el",
			PlatformVersion: "9",
			Architecture:    "x86_64",
			PackageDetail: chefapi.PackageDetail{
				Version: "18.9.4",
				URL:     "https://commercial-acceptance.downloads.chef.co/stable/chef/download?license_id=SECRET&v=18.9.4",
				SHA256:   strings.Repeat("a", 64),
			},
		},
	}

	out := captureStdout(t, func() {
		if err := outputJSON(packages); err != nil {
			t.Fatal(err)
		}
	})

	if strings.Contains(out, "license_id=") {
		t.Fatalf("outputJSON leaked license_id: %s", out)
	}

	var decoded []map[string]any
	if err := json.NewDecoder(bytes.NewBufferString(out)).Decode(&decoded); err != nil {
		t.Fatalf("decoding json: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 package, got %d", len(decoded))
	}
	if decoded[0]["url"] != "" {
		t.Fatalf("expected url to be redacted, got %v", decoded[0]["url"])
	}
}

