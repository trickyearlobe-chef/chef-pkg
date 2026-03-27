package ideconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// makeIDE creates a test IDE definition pointing at a temp directory.
func makeIDE(t *testing.T, name, topLevelKey, configFileName string, createDir bool) IDE {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if createDir {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("creating dir for %s: %v", name, err)
		}
	}
	return IDE{
		Name:        name,
		ConfigDir:   dir,
		ConfigFile:  filepath.Join(dir, configFileName),
		TopLevelKey: topLevelKey,
	}
}

// readJSON reads and parses a JSON file into a map.
func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parsing %s: %v\nContent:\n%s", path, err, string(data))
	}
	return m
}

// writeJSON writes a map as indented JSON to a file.
func writeJSON(t *testing.T, path string, data map[string]any) {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("marshaling: %v", err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestInstallNewFile(t *testing.T) {
	ide := makeIDE(t, "TestIDE", "mcpServers", "config.json", true)

	results := installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "installed" {
		t.Errorf("status = %q, want %q", results[0].Status, "installed")
	}

	config := readJSON(t, ide.ConfigFile)
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers not found or wrong type")
	}
	entry, ok := servers["chef-pkg"].(map[string]any)
	if !ok {
		t.Fatal("chef-pkg entry not found or wrong type")
	}
	if entry["command"] != "/usr/local/bin/chef-pkg" {
		t.Errorf("command = %v, want /usr/local/bin/chef-pkg", entry["command"])
	}
}

func TestInstallPreservesOtherKeys(t *testing.T) {
	ide := makeIDE(t, "TestIDE", "servers", "mcp.json", true)

	// Write existing config with another server and a top-level key
	existing := map[string]any{
		"someOtherKey": "someValue",
		"servers": map[string]any{
			"other-server": map[string]any{
				"command": "/usr/bin/other",
			},
		},
	}
	writeJSON(t, ide.ConfigFile, existing)

	results := installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})

	if results[0].Status != "installed" {
		t.Errorf("status = %q, want %q", results[0].Status, "installed")
	}

	config := readJSON(t, ide.ConfigFile)

	// Check other top-level key preserved
	if config["someOtherKey"] != "someValue" {
		t.Error("someOtherKey was not preserved")
	}

	// Check other server preserved
	servers := config["servers"].(map[string]any)
	if _, ok := servers["other-server"]; !ok {
		t.Error("other-server was not preserved")
	}
	if _, ok := servers["chef-pkg"]; !ok {
		t.Error("chef-pkg was not added")
	}
}

func TestInstallIdempotent(t *testing.T) {
	ide := makeIDE(t, "TestIDE", "mcpServers", "config.json", true)

	// First install
	results := installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})
	if results[0].Status != "installed" {
		t.Fatalf("first install: status = %q, want %q", results[0].Status, "installed")
	}

	// Second install — same path, same args
	results = installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})
	if results[0].Status != "already up to date" {
		t.Errorf("second install: status = %q, want %q", results[0].Status, "already up to date")
	}
}

func TestInstallUpdatesWhenCommandChanges(t *testing.T) {
	ide := makeIDE(t, "TestIDE", "mcpServers", "config.json", true)

	installWithIDEs("chef-pkg", "/old/path/chef-pkg", []string{"serve"}, []IDE{ide})

	results := installWithIDEs("chef-pkg", "/new/path/chef-pkg", []string{"serve"}, []IDE{ide})
	if results[0].Status != "updated" {
		t.Errorf("status = %q, want %q", results[0].Status, "updated")
	}

	config := readJSON(t, ide.ConfigFile)
	servers := config["mcpServers"].(map[string]any)
	entry := servers["chef-pkg"].(map[string]any)
	if entry["command"] != "/new/path/chef-pkg" {
		t.Errorf("command = %v, want /new/path/chef-pkg", entry["command"])
	}
}

func TestInstallUpdatesWhenArgsChange(t *testing.T) {
	ide := makeIDE(t, "TestIDE", "mcpServers", "config.json", true)

	installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})

	results := installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve", "--verbose"}, []IDE{ide})
	if results[0].Status != "updated" {
		t.Errorf("status = %q, want %q", results[0].Status, "updated")
	}
}

func TestInstallSkipsWhenIDENotInstalled(t *testing.T) {
	ide := makeIDE(t, "MissingIDE", "mcpServers", "config.json", false) // dir not created

	results := installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})

	if results[0].Status != "skipped" {
		t.Errorf("status = %q, want %q", results[0].Status, "skipped")
	}
}

func TestInstallDifferentTopLevelKeys(t *testing.T) {
	tests := []struct {
		name        string
		topLevelKey string
	}{
		{"mcpServers", "mcpServers"},
		{"servers", "servers"},
		{"context_servers", "context_servers"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ide := makeIDE(t, tt.name, tt.topLevelKey, "config.json", true)

			installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})

			config := readJSON(t, ide.ConfigFile)
			if _, ok := config[tt.topLevelKey]; !ok {
				t.Errorf("top-level key %q not found in config", tt.topLevelKey)
			}
		})
	}
}

func TestInstallZedIncludesArgsAndEnv(t *testing.T) {
	ide := makeIDE(t, "Zed", "context_servers", "settings.json", true)

	installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})

	config := readJSON(t, ide.ConfigFile)
	servers := config["context_servers"].(map[string]any)
	entry := servers["chef-pkg"].(map[string]any)

	if _, ok := entry["args"]; !ok {
		t.Error("Zed entry missing 'args' field")
	}
	if _, ok := entry["env"]; !ok {
		t.Error("Zed entry missing 'env' field")
	}
}

func TestInstallNonZedDoesNotIncludeEnv(t *testing.T) {
	ide := makeIDE(t, "VSCode", "servers", "mcp.json", true)

	installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})

	config := readJSON(t, ide.ConfigFile)
	servers := config["servers"].(map[string]any)
	entry := servers["chef-pkg"].(map[string]any)

	if _, ok := entry["env"]; ok {
		t.Error("non-Zed entry should not have 'env' field")
	}
}

func TestInstallUpdatesOldZedEntryMissingEnv(t *testing.T) {
	ide := makeIDE(t, "Zed", "context_servers", "settings.json", true)

	// Write an old-style Zed entry that's missing args and env
	existing := map[string]any{
		"context_servers": map[string]any{
			"chef-pkg": map[string]any{
				"command": "/usr/local/bin/chef-pkg",
			},
		},
	}
	writeJSON(t, ide.ConfigFile, existing)

	results := installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})
	if results[0].Status != "updated" {
		t.Errorf("status = %q, want %q (old entry missing env should trigger update)", results[0].Status, "updated")
	}

	config := readJSON(t, ide.ConfigFile)
	servers := config["context_servers"].(map[string]any)
	entry := servers["chef-pkg"].(map[string]any)
	if _, ok := entry["env"]; !ok {
		t.Error("re-installed Zed entry should now have 'env' field")
	}
}

func TestInstallZedJSONC(t *testing.T) {
	ide := makeIDE(t, "Zed", "context_servers", "settings.json", true)

	// Write a JSONC file with comments and trailing commas
	jsonc := `// Zed settings
// Generated by Zed
{
  // Font settings
  "font_size": 14,
  "theme": "One Dark",
  "context_servers": {
    "other-server": {
      "command": "/usr/bin/other",
      "args": [],
      "env": {},
    },
  },
}
`
	if err := os.WriteFile(ide.ConfigFile, []byte(jsonc), 0644); err != nil {
		t.Fatalf("writing JSONC: %v", err)
	}

	results := installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})
	if results[0].Status != "installed" {
		t.Errorf("status = %q, want %q", results[0].Status, "installed")
	}

	// Read back and verify
	data, err := os.ReadFile(ide.ConfigFile)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	content := string(data)

	// Preamble should be preserved
	if content[:15] != "// Zed settings" {
		t.Errorf("preamble not preserved, starts with: %q", content[:30])
	}

	// Should be valid JSON after the preamble
	_, body := ExtractPreamble(content)
	var config map[string]any
	if err := json.Unmarshal([]byte(body), &config); err != nil {
		t.Fatalf("result is not valid JSON: %v\nBody:\n%s", err, body)
	}

	// Other settings preserved
	if config["font_size"] == nil {
		t.Error("font_size was not preserved")
	}
	if config["theme"] == nil {
		t.Error("theme was not preserved")
	}

	// Both servers present
	servers := config["context_servers"].(map[string]any)
	if _, ok := servers["other-server"]; !ok {
		t.Error("other-server was not preserved")
	}
	if _, ok := servers["chef-pkg"]; !ok {
		t.Error("chef-pkg was not installed")
	}
}

func TestUninstallRemovesEntry(t *testing.T) {
	ide := makeIDE(t, "TestIDE", "mcpServers", "config.json", true)

	// Install first
	installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})

	// Uninstall
	results := uninstallWithIDEs("chef-pkg", []IDE{ide})
	if results[0].Status != "removed" {
		t.Errorf("status = %q, want %q", results[0].Status, "removed")
	}

	config := readJSON(t, ide.ConfigFile)
	// The top-level key should be gone since it was the only server
	if _, ok := config["mcpServers"]; ok {
		t.Error("mcpServers should have been removed when last server was deleted")
	}
}

func TestUninstallPreservesOtherServers(t *testing.T) {
	ide := makeIDE(t, "TestIDE", "servers", "mcp.json", true)

	existing := map[string]any{
		"servers": map[string]any{
			"chef-pkg": map[string]any{
				"command": "/usr/local/bin/chef-pkg",
			},
			"other-server": map[string]any{
				"command": "/usr/bin/other",
			},
		},
	}
	writeJSON(t, ide.ConfigFile, existing)

	results := uninstallWithIDEs("chef-pkg", []IDE{ide})
	if results[0].Status != "removed" {
		t.Errorf("status = %q, want %q", results[0].Status, "removed")
	}

	config := readJSON(t, ide.ConfigFile)
	servers := config["servers"].(map[string]any)
	if _, ok := servers["chef-pkg"]; ok {
		t.Error("chef-pkg should have been removed")
	}
	if _, ok := servers["other-server"]; !ok {
		t.Error("other-server should have been preserved")
	}
}

func TestUninstallSkipsWhenIDENotInstalled(t *testing.T) {
	ide := makeIDE(t, "MissingIDE", "mcpServers", "config.json", false)

	results := uninstallWithIDEs("chef-pkg", []IDE{ide})
	if results[0].Status != "skipped" {
		t.Errorf("status = %q, want %q", results[0].Status, "skipped")
	}
}

func TestUninstallNotPresentWhenNoConfigFile(t *testing.T) {
	ide := makeIDE(t, "TestIDE", "mcpServers", "config.json", true)
	// Dir exists but no config file

	results := uninstallWithIDEs("chef-pkg", []IDE{ide})
	if results[0].Status != "not present" {
		t.Errorf("status = %q, want %q", results[0].Status, "not present")
	}
}

func TestUninstallNotPresentWhenServerMissing(t *testing.T) {
	ide := makeIDE(t, "TestIDE", "mcpServers", "config.json", true)

	existing := map[string]any{
		"mcpServers": map[string]any{
			"other-server": map[string]any{
				"command": "/usr/bin/other",
			},
		},
	}
	writeJSON(t, ide.ConfigFile, existing)

	results := uninstallWithIDEs("chef-pkg", []IDE{ide})
	if results[0].Status != "not present" {
		t.Errorf("status = %q, want %q", results[0].Status, "not present")
	}
}

func TestUninstallIdempotent(t *testing.T) {
	ide := makeIDE(t, "TestIDE", "mcpServers", "config.json", true)

	installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})

	// First uninstall
	results := uninstallWithIDEs("chef-pkg", []IDE{ide})
	if results[0].Status != "removed" {
		t.Fatalf("first uninstall: status = %q, want %q", results[0].Status, "removed")
	}

	// Second uninstall — server no longer present
	results = uninstallWithIDEs("chef-pkg", []IDE{ide})
	if results[0].Status != "not present" {
		t.Errorf("second uninstall: status = %q, want %q", results[0].Status, "not present")
	}
}

func TestRoundTripInstallUninstall(t *testing.T) {
	ide := makeIDE(t, "TestIDE", "mcpServers", "config.json", true)

	// Install
	results := installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})
	if results[0].Status != "installed" {
		t.Fatalf("install: status = %q, want %q", results[0].Status, "installed")
	}

	// Verify installed
	config := readJSON(t, ide.ConfigFile)
	servers := config["mcpServers"].(map[string]any)
	if _, ok := servers["chef-pkg"]; !ok {
		t.Fatal("chef-pkg not found after install")
	}

	// Uninstall
	results = uninstallWithIDEs("chef-pkg", []IDE{ide})
	if results[0].Status != "removed" {
		t.Fatalf("uninstall: status = %q, want %q", results[0].Status, "removed")
	}

	// Verify removed
	config = readJSON(t, ide.ConfigFile)
	if _, ok := config["mcpServers"]; ok {
		t.Error("mcpServers should be gone after uninstall")
	}

	// Uninstall again — idempotent
	results = uninstallWithIDEs("chef-pkg", []IDE{ide})
	if results[0].Status != "not present" {
		t.Errorf("second uninstall: status = %q, want %q", results[0].Status, "not present")
	}
}

func TestMultipleIDEs(t *testing.T) {
	claude := makeIDE(t, "Claude", "mcpServers", "config.json", true)
	vscode := makeIDE(t, "VSCode", "servers", "mcp.json", true)
	missing := makeIDE(t, "Missing", "mcpServers", "config.json", false)

	ides := []IDE{claude, vscode, missing}

	results := installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, ides)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Status != "installed" {
		t.Errorf("Claude: status = %q, want %q", results[0].Status, "installed")
	}
	if results[1].Status != "installed" {
		t.Errorf("VSCode: status = %q, want %q", results[1].Status, "installed")
	}
	if results[2].Status != "skipped" {
		t.Errorf("Missing: status = %q, want %q", results[2].Status, "skipped")
	}
}

func TestHasErrors(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		want    bool
	}{
		{
			name:    "no errors",
			results: []Result{{Status: "installed"}, {Status: "skipped"}},
			want:    false,
		},
		{
			name:    "has error",
			results: []Result{{Status: "installed"}, {Status: "ERROR"}},
			want:    true,
		},
		{
			name:    "empty",
			results: []Result{},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasErrors(tt.results); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUninstallZedPreservesPreamble(t *testing.T) {
	ide := makeIDE(t, "Zed", "context_servers", "settings.json", true)

	// Write a JSONC file with preamble
	jsonc := `// Zed settings
{
  "font_size": 14,
  "context_servers": {
    "chef-pkg": {
      "command": "/usr/local/bin/chef-pkg",
      "args": ["serve"],
      "env": {}
    }
  }
}
`
	if err := os.WriteFile(ide.ConfigFile, []byte(jsonc), 0644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	results := uninstallWithIDEs("chef-pkg", []IDE{ide})
	if results[0].Status != "removed" {
		t.Errorf("status = %q, want %q", results[0].Status, "removed")
	}

	data, err := os.ReadFile(ide.ConfigFile)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	content := string(data)

	// Preamble preserved
	if content[:15] != "// Zed settings" {
		t.Errorf("preamble not preserved, starts with: %q", content[:30])
	}

	// font_size preserved, context_servers removed
	_, body := ExtractPreamble(content)
	var config map[string]any
	if err := json.Unmarshal([]byte(body), &config); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if config["font_size"] == nil {
		t.Error("font_size should be preserved")
	}
	if _, ok := config["context_servers"]; ok {
		t.Error("context_servers should be removed (was the only server)")
	}
}

func TestInstallCreatesParentDirs(t *testing.T) {
	// IDE config dir exists, but the config file is in a subdirectory
	// that doesn't exist yet. The installer should create it.
	base := t.TempDir()
	configDir := filepath.Join(base, "ide")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	ide := IDE{
		Name:        "TestIDE",
		ConfigDir:   configDir,
		ConfigFile:  filepath.Join(configDir, "subdir", "config.json"),
		TopLevelKey: "mcpServers",
	}

	results := installWithIDEs("chef-pkg", "/usr/local/bin/chef-pkg", []string{"serve"}, []IDE{ide})
	if results[0].Status != "installed" {
		t.Errorf("status = %q, want %q (message: %s)", results[0].Status, "installed", results[0].Message)
	}

	if _, err := os.Stat(ide.ConfigFile); err != nil {
		t.Errorf("config file should have been created: %v", err)
	}
}
