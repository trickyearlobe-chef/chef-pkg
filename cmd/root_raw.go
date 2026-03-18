package cmd

import (
	"fmt"
	"os"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

var rawCmd = &cobra.Command{
	Use:   "raw",
	Short: "Explore the Chef downloads API",
	Long: `Send raw GET requests to the Chef downloads API and print the response body.

This is useful for discovering endpoints, inspecting payloads, and debugging
API behavior without going through a higher-level subcommand.`,
}

var rawGetCmd = &cobra.Command{
	Use:   "get <path>",
	Short: "GET an arbitrary API path",
	Args:  cobra.ExactArgs(1),
	RunE:  runRawGet,
}

func init() {
	rootCmd.AddCommand(rawCmd)
	rawCmd.AddCommand(rawGetCmd)

	rawGetCmd.Flags().StringSlice("query", nil, "Optional query parameter in key=value form; may be repeated")
}

func runRawGet(cmd *cobra.Command, args []string) error {
	licenseID := viper.GetString("chef.license_id")
	if licenseID == "" {
		return fmt.Errorf("license ID is required: set --license-id, config chef.license_id, or CHEFPKG_CHEF_LICENSE_ID env var")
	}

	baseURL := viper.GetString("chef.base_url")
	if baseURL == "" {
		baseURL = "https://commercial-acceptance.downloads.chef.co"
	}

	clientOpts := []chefapi.ClientOption{}
	if baseURL != "" {
		clientOpts = append(clientOpts, chefapi.WithBaseURL(strings.TrimRight(baseURL, "/")))
	}
	client := chefapi.NewClient(licenseID, clientOpts...)

	params := map[string]string{}
	for _, raw := range getStringSliceFlag(cmd, "query") {
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			return fmt.Errorf("invalid --query %q: expected key=value", raw)
		}
		params[parts[0]] = parts[1]
	}

	body, err := client.RawGet(cmd.Context(), stripWrappingQuotes(args[0]), mapToURLValues(params))
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(body)
	if err == nil && len(body) > 0 && body[len(body)-1] != '\n' {
		_, err = os.Stdout.Write([]byte("\n"))
	}
	return err
}

func mapToURLValues(values map[string]string) url.Values {
	out := make(url.Values, len(values))
	for k, v := range values {
		out.Set(k, v)
	}
	return out
}

func getStringSliceFlag(cmd *cobra.Command, name string) []string {
	vals, _ := cmd.Flags().GetStringSlice(name)
	return vals
}

func stripWrappingQuotes(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}
	return value
}
