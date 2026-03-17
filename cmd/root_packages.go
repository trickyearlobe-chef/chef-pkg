package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

var packagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "List available packages for a Chef product",
	Long: `Fetch and display the available packages for a given Chef product,
version, and channel from the Progress Chef commercial downloads API.

Results can be filtered by platform and architecture, and displayed
as a table or as JSON.`,
	Example: `  # List all packages for chef-ice 19.1.158
  chef-pkg packages --product chef-ice --version 19.1.158

  # Filter by platform
  chef-pkg packages --product chef-ice --version 19.1.158 --platform ubuntu

  # Output as JSON
  chef-pkg packages --product chef-ice --version 19.1.158 --output json`,
	RunE: runPackages,
}

func init() {
	rootCmd.AddCommand(packagesCmd)

	packagesCmd.Flags().StringP("product", "p", "chef-ice", "Chef product name (e.g. chef-ice, chef, inspec)")
	packagesCmd.Flags().StringP("version", "v", "", "Product version to fetch (required)")
	packagesCmd.Flags().String("platform", "", "Filter results by platform (substring match, case-insensitive)")
	packagesCmd.Flags().String("arch", "", "Filter results by architecture (substring match, case-insensitive)")
	packagesCmd.Flags().StringP("output", "o", "table", "Output format: table or json")

	_ = packagesCmd.MarkFlagRequired("version")
}

func runPackages(cmd *cobra.Command, args []string) error {
	licenseID := viper.GetString("chef.license_id")
	if licenseID == "" {
		return fmt.Errorf("license ID is required: set --license-id, config chef.license_id, or CHEFPKG_CHEF_LICENSE_ID env var")
	}

	baseURL := viper.GetString("chef.base_url")
	channel := viper.GetString("chef.channel")
	product, _ := cmd.Flags().GetString("product")
	version, _ := cmd.Flags().GetString("version")
	platform, _ := cmd.Flags().GetString("platform")
	arch, _ := cmd.Flags().GetString("arch")
	output, _ := cmd.Flags().GetString("output")

	var opts []chefapi.ClientOption
	if baseURL != "" {
		opts = append(opts, chefapi.WithBaseURL(baseURL))
	}

	client := chefapi.NewClient(licenseID, opts...)

	resp, err := client.FetchPackages(cmd.Context(), channel, product, version)
	if err != nil {
		return fmt.Errorf("fetching packages: %w", err)
	}

	packages := resp.Flatten()
	packages = filterPackages(packages, platform, arch)

	if len(packages) == 0 {
		fmt.Fprintln(os.Stderr, "No packages found matching the specified criteria.")
		return nil
	}

	switch strings.ToLower(output) {
	case "json":
		return outputJSON(packages)
	case "table":
		return outputTable(packages)
	default:
		return fmt.Errorf("unknown output format %q: use 'table' or 'json'", output)
	}
}

func filterPackages(packages []chefapi.FlatPackage, platform, arch string) []chefapi.FlatPackage {
	if platform == "" && arch == "" {
		return packages
	}

	var filtered []chefapi.FlatPackage
	for _, pkg := range packages {
		if platform != "" && !strings.Contains(strings.ToLower(pkg.Platform), strings.ToLower(platform)) {
			continue
		}
		if arch != "" && !strings.Contains(strings.ToLower(pkg.Architecture), strings.ToLower(arch)) {
			continue
		}
		filtered = append(filtered, pkg)
	}
	return filtered
}

func outputTable(packages []chefapi.FlatPackage) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PLATFORM\tVERSION\tARCH\tPACKAGE VERSION\tURL\tSHA256")
	fmt.Fprintln(w, "--------\t-------\t----\t---------------\t---\t------")
	for _, pkg := range packages {
		sha := pkg.SHA256
		if len(sha) > 12 {
			sha = sha[:12] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			pkg.Platform,
			pkg.PlatformVersion,
			pkg.Architecture,
			pkg.Version,
			pkg.URL,
			sha,
		)
	}
	return w.Flush()
}

func outputJSON(packages []chefapi.FlatPackage) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(packages)
}
