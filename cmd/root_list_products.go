package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"

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

  # Include end-of-life products
  chef-pkg list products --include-eol

  # Output as JSON
  chef-pkg list products --output json`,
	RunE: runListProducts,
}

func init() {
	listCmd.AddCommand(listProductsCmd)
	listProductsCmd.Flags().StringP("output", "o", "table", "Output format: table or json")
	listProductsCmd.Flags().Bool("include-eol", false, "Include end-of-life products and show their status")
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
	output, _ := cmd.Flags().GetString("output")
	includeObsolete, _ := cmd.Flags().GetBool("include-eol")

	if !includeObsolete {
		products, err := client.FetchProducts(cmd.Context(), nil)
		if err != nil {
			return fmt.Errorf("fetching products: %w", err)
		}
		return outputProducts(products, output)
	}

	current, err := client.FetchProducts(cmd.Context(), nil)
	if err != nil {
		return fmt.Errorf("fetching current products: %w", err)
	}
	currentSet := make(map[string]bool, len(current))
	for _, p := range current {
		currentSet[p] = true
	}

	all, err := client.FetchProducts(cmd.Context(), url.Values{"eol": {"true"}})
	if err != nil {
		return fmt.Errorf("fetching all products: %w", err)
	}

	results := make([]productStatus, 0, len(all))
	for _, p := range all {
		status := "current"
		if !currentSet[p] {
			status = "eol"
		}
		results = append(results, productStatus{Product: p, Status: status})
	}

	return outputProductStatuses(results, output)
}

func outputProducts(products []string, format string) error {
	switch strings.ToLower(format) {
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
		return fmt.Errorf("unknown output format %q: use 'table' or 'json'", format)
	}
}

type productStatus struct {
	Product string `json:"product"`
	Status  string `json:"status"`
}

func outputProductStatuses(results []productStatus, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "PRODUCT\tSTATUS")
		fmt.Fprintln(w, "-------\t------")
		for _, r := range results {
			fmt.Fprintf(w, "%s\t%s\n", r.Product, r.Status)
		}
		return w.Flush()
	default:
		return fmt.Errorf("unknown output format %q: use 'table' or 'json'", format)
	}
}
