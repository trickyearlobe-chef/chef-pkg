package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

var listVersionsCmd = &cobra.Command{
	Use:   "versions",
	Short: "List available versions for a Chef product",
	Long:  `List available versions for a given Chef product and channel from the Progress Chef commercial downloads API.`,
	Example: `  # List versions for chef (default product)
  chef-pkg list versions

  # List versions for a specific product
  chef-pkg list versions --product chef-ice

  # List versions on the stable channel
  chef-pkg list versions --product chef --channel stable

  # Output as JSON
  chef-pkg list versions --product chef --output json`,
	RunE: runListVersions,
}

func init() {
	listCmd.AddCommand(listVersionsCmd)
	listVersionsCmd.Flags().StringP("product", "p", "chef", "Chef product name (e.g. chef, chef-ice, inspec)")
	listVersionsCmd.Flags().StringP("output", "o", "table", "Output format: table or json")
}

func runListVersions(cmd *cobra.Command, args []string) error {
	licenseID := viper.GetString("chef.license_id")
	if licenseID == "" {
		return fmt.Errorf("license ID is required: set --license-id, config chef.license_id, or CHEFPKG_CHEF_LICENSE_ID env var")
	}

	baseURL := viper.GetString("chef.base_url")
	channel := viper.GetString("chef.channel")
	product, _ := cmd.Flags().GetString("product")
	output, _ := cmd.Flags().GetString("output")

	var opts []chefapi.ClientOption
	if baseURL != "" {
		opts = append(opts, chefapi.WithBaseURL(baseURL))
	}

	client := chefapi.NewClient(licenseID, opts...)

	versions, err := client.FetchVersions(cmd.Context(), channel, product)
	if err != nil {
		return fmt.Errorf("fetching versions: %w", err)
	}

	if len(versions) == 0 {
		fmt.Fprintln(os.Stderr, "No versions found.")
		return nil
	}

	switch strings.ToLower(output) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(versions)
	case "table":
		fmt.Println("VERSION")
		fmt.Println("-------")
		for _, v := range versions {
			fmt.Println(v)
		}
		return nil
	default:
		return fmt.Errorf("unknown output format %q: use 'table' or 'json'", output)
	}
}
