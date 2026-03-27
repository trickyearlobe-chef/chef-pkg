package ideconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// IDE represents a supported IDE with its config file details.
type IDE struct {
	Name        string
	ConfigDir   string // directory that must exist to consider the IDE installed
	ConfigFile  string // path to the config file
	TopLevelKey string // JSON key under which servers are registered
}

// Result describes the outcome of an install/uninstall operation for one IDE.
type Result struct {
	IDE     string
	Status  string // "installed", "updated", "already up to date", "skipped", "removed", "not present", "ERROR"
	Message string // additional detail for errors
}

// ServerEntry is the JSON structure written into IDE config files.
type ServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// DetectIDEs returns the list of supported IDEs with platform-specific paths.
func DetectIDEs() []IDE {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	ides := []IDE{
		{
			Name:        "Claude Desktop",
			ConfigDir:   claudeConfigDir(home),
			ConfigFile:  filepath.Join(claudeConfigDir(home), "claude_desktop_config.json"),
			TopLevelKey: "mcpServers",
		},
		{
			Name:        "VS Code",
			ConfigDir:   filepath.Join(home, ".vscode"),
			ConfigFile:  filepath.Join(home, ".vscode", "mcp.json"),
			TopLevelKey: "servers",
		},
		{
			Name:        "Cursor",
			ConfigDir:   filepath.Join(home, ".cursor"),
			ConfigFile:  filepath.Join(home, ".cursor", "mcp.json"),
			TopLevelKey: "servers",
		},
		{
			Name:        "Windsurf",
			ConfigDir:   filepath.Join(home, ".codeium", "windsurf"),
			ConfigFile:  filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"),
			TopLevelKey: "mcpServers",
		},
		{
			Name:        "Zed",
			ConfigDir:   filepath.Join(home, ".config", "zed"),
			ConfigFile:  filepath.Join(home, ".config", "zed", "settings.json"),
			TopLevelKey: "context_servers",
		},
	}

	return ides
}

// claudeConfigDir returns the Claude Desktop config directory for the current platform.
func claudeConfigDir(home string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Claude")
	default: // linux and others
		return filepath.Join(home, ".config", "Claude")
	}
}

// Install registers chef-pkg as an MCP server in all detected IDE configs.
// The serverName is the key used in the config (e.g. "chef-pkg").
// The binaryPath is the absolute path to the chef-pkg binary.
// The args are the arguments to pass (e.g. ["serve"]).
func Install(serverName, binaryPath string, args []string) []Result {
	return installWithIDEs(serverName, binaryPath, args, DetectIDEs())
}

// installWithIDEs is the testable implementation of Install.
func installWithIDEs(serverName, binaryPath string, args []string, ides []IDE) []Result {
	var results []Result

	for _, ide := range ides {
		result := installOne(ide, serverName, binaryPath, args)
		results = append(results, result)
	}

	return results
}

// Uninstall removes chef-pkg from all detected IDE configs.
func Uninstall(serverName string) []Result {
	return uninstallWithIDEs(serverName, DetectIDEs())
}

// uninstallWithIDEs is the testable implementation of Uninstall.
func uninstallWithIDEs(serverName string, ides []IDE) []Result {
	var results []Result

	for _, ide := range ides {
		result := uninstallOne(ide, serverName)
		results = append(results, result)
	}

	return results
}

func installOne(ide IDE, serverName, binaryPath string, args []string) Result {
	// Check if IDE is installed
	if _, err := os.Stat(ide.ConfigDir); os.IsNotExist(err) {
		return Result{IDE: ide.Name, Status: "skipped", Message: "not installed"}
	}

	// Read existing config or start fresh
	config, preamble, err := readConfig(ide)
	if err != nil {
		return Result{IDE: ide.Name, Status: "ERROR", Message: err.Error()}
	}

	// Get or create the servers map
	serversRaw, exists := config[ide.TopLevelKey]
	var servers map[string]any
	if exists {
		var ok bool
		servers, ok = serversRaw.(map[string]any)
		if !ok {
			servers = make(map[string]any)
		}
	} else {
		servers = make(map[string]any)
	}

	// Build the entry
	entry := buildEntry(ide, binaryPath, args)

	// Check existing entry
	if existingRaw, ok := servers[serverName]; ok {
		if isUpToDate(existingRaw, entry) {
			return Result{IDE: ide.Name, Status: "already up to date"}
		}
		servers[serverName] = entry
		config[ide.TopLevelKey] = servers
		if err := writeConfig(ide, config, preamble); err != nil {
			return Result{IDE: ide.Name, Status: "ERROR", Message: err.Error()}
		}
		return Result{IDE: ide.Name, Status: "updated"}
	}

	// New entry
	servers[serverName] = entry
	config[ide.TopLevelKey] = servers
	if err := writeConfig(ide, config, preamble); err != nil {
		return Result{IDE: ide.Name, Status: "ERROR", Message: err.Error()}
	}
	return Result{IDE: ide.Name, Status: "installed"}
}

func uninstallOne(ide IDE, serverName string) Result {
	// Check if IDE is installed
	if _, err := os.Stat(ide.ConfigDir); os.IsNotExist(err) {
		return Result{IDE: ide.Name, Status: "skipped", Message: "not installed"}
	}

	// Check if config file exists
	if _, err := os.Stat(ide.ConfigFile); os.IsNotExist(err) {
		return Result{IDE: ide.Name, Status: "not present"}
	}

	config, preamble, err := readConfig(ide)
	if err != nil {
		return Result{IDE: ide.Name, Status: "ERROR", Message: err.Error()}
	}

	serversRaw, exists := config[ide.TopLevelKey]
	if !exists {
		return Result{IDE: ide.Name, Status: "not present"}
	}

	servers, ok := serversRaw.(map[string]any)
	if !ok {
		return Result{IDE: ide.Name, Status: "not present"}
	}

	if _, ok := servers[serverName]; !ok {
		return Result{IDE: ide.Name, Status: "not present"}
	}

	delete(servers, serverName)

	// If the servers map is now empty, remove the top-level key
	if len(servers) == 0 {
		delete(config, ide.TopLevelKey)
	} else {
		config[ide.TopLevelKey] = servers
	}

	if err := writeConfig(ide, config, preamble); err != nil {
		return Result{IDE: ide.Name, Status: "ERROR", Message: err.Error()}
	}

	return Result{IDE: ide.Name, Status: "removed"}
}

// readConfig reads and parses the IDE config file. For Zed, handles JSONC.
// Returns the parsed config map and any preamble (for Zed).
func readConfig(ide IDE) (map[string]any, string, error) {
	data, err := os.ReadFile(ide.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), "", nil
		}
		return nil, "", fmt.Errorf("reading %s: %w", ide.ConfigFile, err)
	}

	raw := string(data)
	var preamble string

	if isZed(ide) {
		preamble, raw = ExtractPreamble(raw)
		raw = StripComments(raw)
	}

	if strings.TrimSpace(raw) == "" {
		return make(map[string]any), preamble, nil
	}

	var config map[string]any
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return nil, "", fmt.Errorf("parsing %s: %w", ide.ConfigFile, err)
	}

	return config, preamble, nil
}

// writeConfig marshals and writes the config back, preserving any preamble.
func writeConfig(ide IDE, config map[string]any, preamble string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(ide.ConfigFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	content := preamble + string(data) + "\n"

	if err := os.WriteFile(ide.ConfigFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", ide.ConfigFile, err)
	}

	return nil
}

// buildEntry creates the server entry for an IDE config.
// Zed requires args and env fields; others just need command and args.
func buildEntry(ide IDE, binaryPath string, args []string) map[string]any {
	entry := map[string]any{
		"command": binaryPath,
	}

	if len(args) > 0 {
		entry["args"] = args
	}

	// Zed requires args and env to be present
	if isZed(ide) {
		if _, ok := entry["args"]; !ok {
			entry["args"] = []string{}
		}
		entry["env"] = map[string]string{}
	}

	return entry
}

// isZed returns true if the IDE is Zed (uses context_servers key).
func isZed(ide IDE) bool {
	return ide.TopLevelKey == "context_servers"
}

// isUpToDate checks whether an existing entry matches the desired entry.
func isUpToDate(existingRaw any, desired map[string]any) bool {
	existing, ok := existingRaw.(map[string]any)
	if !ok {
		return false
	}

	// Check command
	if fmt.Sprintf("%v", existing["command"]) != fmt.Sprintf("%v", desired["command"]) {
		return false
	}

	// Check args
	if !argsMatch(existing["args"], desired["args"]) {
		return false
	}

	// For Zed entries, check env is present
	if _, wantEnv := desired["env"]; wantEnv {
		if _, hasEnv := existing["env"]; !hasEnv {
			return false
		}
	}

	return true
}

// argsMatch compares two args values which may be []any or []string.
func argsMatch(a, b any) bool {
	aList := toStringSlice(a)
	bList := toStringSlice(b)

	if len(aList) != len(bList) {
		return false
	}
	for i := range aList {
		if aList[i] != bList[i] {
			return false
		}
	}
	return true
}

// toStringSlice normalises an args value to []string.
func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		out := make([]string, len(val))
		for i, item := range val {
			out[i] = fmt.Sprintf("%v", item)
		}
		return out
	default:
		return nil
	}
}

// ResolveBinaryPath returns the absolute path to the current executable.
func ResolveBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving executable path: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks for %s: %w", exe, err)
	}

	return resolved, nil
}

// HasErrors returns true if any result has status "ERROR".
func HasErrors(results []Result) bool {
	for _, r := range results {
		if r.Status == "ERROR" {
			return true
		}
	}
	return false
}
