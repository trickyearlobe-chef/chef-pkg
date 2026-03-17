package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configEntry maps a CLI flag name to its Viper config key.
type configEntry struct {
	flag string
	key  string
}

// configEntries defines all configurable keys and their flag names.
var configEntries = []configEntry{
	{"license-id", "chef.license_id"},
	{"cfg-base-url", "chef.base_url"},
	{"cfg-channel", "chef.channel"},
	{"download-dest", "download.dest"},
	{"download-concurrency", "download.concurrency"},
	{"nexus-url", "nexus.url"},
	{"nexus-username", "nexus.username"},
	{"nexus-password", "nexus.password"},
	{"artifactory-url", "artifactory.url"},
	{"artifactory-token", "artifactory.token"},
	{"artifactory-username", "artifactory.username"},
	{"artifactory-password", "artifactory.password"},
}

// secretSuffixes are key suffixes that indicate a secret value.
var secretSuffixes = []string{"license_id", "password", "token"}

var showConfig bool

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Set or display configuration values",
	Long: `Set or display configuration values in the config file.

When called with flags, updates the config file with the specified values.
When called with --show, displays the current resolved configuration
with secrets masked.`,
	Example: `  # Set your license ID
  chef-pkg configure --license-id xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

  # Set multiple values at once
  chef-pkg configure --nexus-url https://nexus.example.com --nexus-username admin

  # Show current configuration
  chef-pkg configure --show`,
	RunE: runConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)

	configureCmd.Flags().String("license-id", "", "Chef license ID")
	configureCmd.Flags().String("cfg-base-url", "", "Base URL of the Chef downloads API")
	configureCmd.Flags().String("cfg-channel", "", "Release channel")
	configureCmd.Flags().String("download-dest", "", "Download destination directory")
	configureCmd.Flags().Int("download-concurrency", 0, "Max parallel downloads")
	configureCmd.Flags().String("nexus-url", "", "Nexus server URL")
	configureCmd.Flags().String("nexus-username", "", "Nexus username")
	configureCmd.Flags().String("nexus-password", "", "Nexus password")
	configureCmd.Flags().String("artifactory-url", "", "Artifactory server URL")
	configureCmd.Flags().String("artifactory-token", "", "Artifactory API token")
	configureCmd.Flags().String("artifactory-username", "", "Artifactory username")
	configureCmd.Flags().String("artifactory-password", "", "Artifactory password")
	configureCmd.Flags().BoolVar(&showConfig, "show", false, "Display current resolved config")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	if showConfig {
		return showCurrentConfig()
	}

	// Collect flags that were explicitly set
	updates := map[string]interface{}{}
	for _, entry := range configEntries {
		f := cmd.Flags().Lookup(entry.flag)
		if f == nil || !f.Changed {
			continue
		}
		switch f.Value.Type() {
		case "int":
			v, _ := cmd.Flags().GetInt(entry.flag)
			updates[entry.key] = v
		default:
			v, _ := cmd.Flags().GetString(entry.flag)
			updates[entry.key] = v
		}
	}

	if len(updates) == 0 {
		return fmt.Errorf("no configuration values specified; use flags to set values or --show to display current config")
	}

	return writeConfig(updates)
}

// configFilePath returns the path to the config file.
func configFilePath() string {
	if cfgFile != "" {
		return cfgFile
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".chef-pkg.toml"
	}
	return filepath.Join(home, ".chef-pkg.toml")
}

// writeConfig reads the existing config file (if any), merges updates, and writes it back.
func writeConfig(updates map[string]interface{}) error {
	path := configFilePath()

	// Read existing config into a nested map
	config := map[string]interface{}{}
	data, err := os.ReadFile(path)
	if err == nil {
		_ = toml.Unmarshal(data, &config)
	}

	// Apply updates: keys are dotted like "chef.license_id"
	for key, val := range updates {
		parts := strings.SplitN(key, ".", 2)
		if len(parts) == 2 {
			section, field := parts[0], parts[1]
			sec, ok := config[section]
			if !ok {
				sec = map[string]interface{}{}
			}
			secMap, ok := sec.(map[string]interface{})
			if !ok {
				secMap = map[string]interface{}{}
			}
			secMap[field] = val
			config[section] = secMap
		} else {
			config[key] = val
		}
	}

	out, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	if err := os.WriteFile(path, out, 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Config written to %s\n", path)
	return nil
}

// showCurrentConfig prints all config values as a table, masking secrets.
func showCurrentConfig() error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tVALUE\tSOURCE")
	fmt.Fprintln(w, "---\t-----\t------")

	// Gather all known keys and sort them
	keys := make([]string, 0, len(configEntries))
	for _, entry := range configEntries {
		keys = append(keys, entry.key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		val := viper.GetString(key)
		source := configSource(key)
		if val != "" && isSecretKey(key) {
			val = maskSecret(val)
		}
		if val == "" {
			val = "(not set)"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", key, val, source)
	}
	return w.Flush()
}

// configSource returns a human-readable description of where a config value came from.
func configSource(key string) string {
	// Check if the value is set in environment
	envKey := "CHEFPKG_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	if _, ok := os.LookupEnv(envKey); ok {
		return "env: " + envKey
	}
	if viper.ConfigFileUsed() != "" && viper.IsSet(key) {
		return "file: " + viper.ConfigFileUsed()
	}
	return "default"
}

// maskSecret masks a secret value for display.
// Values >= 10 chars show first 4 + **** + last 4.
// Shorter non-empty values show ****.
// Empty values are returned as-is.
func maskSecret(val string) string {
	if val == "" {
		return ""
	}
	if len(val) >= 10 {
		return val[:4] + "****" + val[len(val)-4:]
	}
	return "****"
}

// isSecretKey returns true if the config key refers to a secret value.
func isSecretKey(key string) bool {
	for _, suffix := range secretSuffixes {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}
