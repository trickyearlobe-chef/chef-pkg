package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	internalmcp "github.com/trickyearlobe-chef/chef-pkg/internal/mcp"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server over stdio transport",
	Long: `Start chef-pkg as an MCP (Model Context Protocol) server communicating
over stdio transport. This allows AI assistants to query the Chef downloads
API using the registered tools.

The server requires a valid license ID, which can be provided via config file,
environment variable, or the --license-id flag.

Use 'chef-pkg --install' to register this server with supported IDEs.`,
	Example: `  # Start the MCP server
  chef-pkg serve

  # Start with a specific license ID
  chef-pkg serve --license-id xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

  # Start with a custom base URL
  chef-pkg serve --base-url https://commercial-acceptance.downloads.chef.co`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	// NEVER write to stdout — MCP owns it. Redirect the default logger.
	log.SetOutput(os.Stderr)

	licenseID := viper.GetString("chef.license_id")
	if licenseID == "" {
		return fmt.Errorf("license ID is required: set --license-id, config chef.license_id, or CHEFPKG_CHEF_LICENSE_ID env var")
	}

	baseURL := viper.GetString("chef.base_url")
	var clientOpts []chefapi.ClientOption
	if baseURL != "" {
		clientOpts = append(clientOpts, chefapi.WithBaseURL(baseURL))
	}

	client := chefapi.NewClient(licenseID, clientOpts...)

	channel := viper.GetString("chef.channel")
	if channel == "" {
		channel = "stable"
	}

	cfg := internalmcp.ServerConfig{
		Version: version,
		Channel: channel,
		Client:  client,
	}

	return internalmcp.Run(cmd.Context(), cfg)
}
