package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available packages for a Chef product",
	Long: `Fetch and display the available packages for a given Chef product,
version, and channel from the Progress Chef commercial downloads API.

Results can be filtered by platform and architecture, and displayed
as a table or as JSON.

The --version flag accepts a semver version (e.g. 18.4.12), "latest" to
resolve the most recent version, or "all" to list packages for every
available version. When using "latest" with --platform or --arch filters,
the latest version that has matching packages is selected.

Use subcommands to list products or versions instead.`,
	Example: `  # List all packages for chef-ice 19.1.158
  chef-pkg list --product chef-ice --version 19.1.158

  # List packages for the latest version
  chef-pkg list --product chef --version latest

  # List latest packages available for Ubuntu
  chef-pkg list --product chef --version latest --platform ubuntu

  # List packages across all versions (filtered to aarch64)
  chef-pkg list --product chef --version all --arch aarch64

  # Filter by platform
  chef-pkg list --product chef-ice --version 19.1.158 --platform ubuntu

  # Output as JSON
  chef-pkg list --version 19.1.158 --output json`,
	RunE: runListPackages,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringP("product", "p", "chef", "Chef product name (e.g. chef, chef-ice, inspec)")
	listCmd.Flags().StringP("version", "v", "latest", "Product version: semver (e.g. 18.4.12), 'latest', or 'all' (default: latest)")
	listCmd.Flags().String("platform", "", "Filter results by platform (substring match, case-insensitive)")
	listCmd.Flags().String("arch", "", "Filter results by architecture (substring match, case-insensitive)")
	listCmd.Flags().StringP("output", "o", "table", "Output format: table or json")
}

func runListPackages(cmd *cobra.Command, args []string) error {
	licenseID := viper.GetString("chef.license_id")
	if licenseID == "" {
		return fmt.Errorf("license ID is required: set --license-id, config chef.license_id, or CHEFPKG_CHEF_LICENSE_ID env var")
	}

	baseURL := viper.GetString("chef.base_url")
	channel := viper.GetString("chef.channel")
	product, _ := cmd.Flags().GetString("product")
	versionFlag, _ := cmd.Flags().GetString("version")
	platform, _ := cmd.Flags().GetString("platform")
	arch, _ := cmd.Flags().GetString("arch")
	output, _ := cmd.Flags().GetString("output")

	if versionFlag == "" {
		versionFlag = "latest"
	}

	var opts []chefapi.ClientOption
	if baseURL != "" {
		opts = append(opts, chefapi.WithBaseURL(baseURL))
	}

	client := chefapi.NewClient(licenseID, opts...)

	// Resolve version(s)
	versions, err := resolveVersions(cmd.Context(), client, channel, product, versionFlag, platform, arch)
	if err != nil {
		return err
	}

	// Collect packages across all resolved versions
	var allPackages []chefapi.FlatPackage

	for _, version := range versions {
		if strings.EqualFold(versionFlag, "all") {
			fmt.Fprintf(os.Stderr, "Fetching packages for %s %s...\n", product, version)
		}

		resp, err := client.FetchPackages(cmd.Context(), channel, product, version)
		if err != nil {
			if strings.EqualFold(versionFlag, "all") {
				fmt.Fprintf(os.Stderr, "  Warning: skipping version %s: %v\n", version, err)
				continue
			}
			return fmt.Errorf("fetching packages for version %s: %w", version, err)
		}

		packages := resp.Flatten(product)
		packages = filterPackages(packages, platform, arch)
		allPackages = append(allPackages, packages...)
	}

	if len(allPackages) == 0 {
		fmt.Fprintln(os.Stderr, "No packages found matching the specified criteria.")
		return nil
	}

	switch strings.ToLower(output) {
	case "json":
		return outputJSON(allPackages)
	case "table":
		return outputTable(allPackages)
	default:
		return fmt.Errorf("unknown output format %q: use 'table' or 'json'", output)
	}
}
