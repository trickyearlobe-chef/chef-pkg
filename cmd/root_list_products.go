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

var listProductsCmd = &cobra.Command{
	Use:   "products",
	Short: "List available Chef products",
	Long:  `List all available products from the Progress Chef commercial downloads API.`,
	Example: `  # List all products
  chef-pkg list products

  # Output as JSON
  chef-pkg list products --output json`,
	RunE: runListProducts,
}

func init() {
	listCmd.AddCommand(listProductsCmd)
	listProductsCmd.Flags().StringP("output", "o", "table", "Output format: table or json")
}

func runListProducts(cmd *cobra.Command, args []string) error {
	licenseID := viper.GetString("chef.license_id")
	if licenseID == "" {
		return fmt.Errorf("license ID is required: set --license-id, config chef.license_id, or CHEFPKG_CHEF_LICENSE_ID env var")
	}

	baseURL := viper.GetString("chef.base_url")
	var opts []chefapi.ClientOption
	if baseURL != "" {
		opts = append(opts, chefapi.WithBaseURL(baseURL))
	}

	client := chefapi.NewClient(licenseID, opts...)

	products, err := client.FetchProducts(cmd.Context())
	if err != nil {
		return fmt.Errorf("fetching products: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	switch strings.ToLower(output) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(products)
	case "table":
		fmt.Println("PRODUCT")
		fmt.Println("-------")
		for _, p := range products {
			fmt.Println(p)
		}
		return nil
	default:
		return fmt.Errorf("unknown output format %q: use 'table' or 'json'", output)
	}
}
