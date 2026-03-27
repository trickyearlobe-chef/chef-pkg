package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

// ServerConfig holds the configuration needed to create an MCP server.
type ServerConfig struct {
	// Version is the server version string (set at build time).
	Version string
	// Channel is the default release channel (e.g. "stable").
	Channel string
	// Client is the configured Chef API client.
	Client *chefapi.Client
}

// NewServer creates a configured MCP server with all tools registered.
func NewServer(cfg ServerConfig) *sdkmcp.Server {
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	if cfg.Channel == "" {
		cfg.Channel = "stable"
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{
			Name:    "chef-pkg",
			Version: cfg.Version,
		},
		&sdkmcp.ServerOptions{
			Instructions: "You are a Chef package discovery assistant. Use the available tools to help users find Chef products, versions, and packages across platforms and architectures.",
			Logger:       logger,
		},
	)

	registerTools(server, cfg.Client, cfg.Channel)

	return server
}

// registerTools adds all MCP tools to the server.
func registerTools(server *sdkmcp.Server, client *chefapi.Client, defaultChannel string) {
	sdkmcp.AddTool[RawGetInput, RawGetOutput](server, &sdkmcp.Tool{
		Name:        "raw_get",
		Description: "Send a raw GET request to any Chef downloads API endpoint. Useful for exploring the API, discovering endpoints, and inspecting payloads. The license_id query parameter is added automatically. Returns the parsed JSON response body, or a raw string if the response is not valid JSON.",
	}, rawGetHandler(client))

	sdkmcp.AddTool[ListProductsInput, ListProductsOutput](server, &sdkmcp.Tool{
		Name:        "list_products",
		Description: "List all available Chef products. Returns product names with lifecycle status (current or eol). Use include_eol to also see end-of-life products like chefdk and analytics.",
	}, listProductsHandler(client))

	sdkmcp.AddTool[ListVersionsInput, ListVersionsOutput](server, &sdkmcp.Tool{
		Name:        "list_versions",
		Description: "List available versions for a Chef product in ascending order. Defaults to product 'chef' on the 'stable' channel if not specified.",
	}, listVersionsHandler(client, defaultChannel))

	sdkmcp.AddTool[ListPackagesInput, ListPackagesOutput](server, &sdkmcp.Tool{
		Name:        "list_packages",
		Description: "List available packages for a Chef product version across platforms and architectures. Defaults to the latest version of 'chef' on the 'stable' channel. Use platform and arch filters to narrow results (case-insensitive substring match).",
	}, listPackagesHandler(client, defaultChannel))
}

// Run starts the MCP server on stdio transport and blocks until the client
// disconnects or the context is cancelled.
func Run(ctx context.Context, cfg ServerConfig) error {
	server := NewServer(cfg)

	fmt.Fprintln(os.Stderr, "chef-pkg MCP server starting on stdio transport...")

	if err := server.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
		return fmt.Errorf("mcp server: %w", err)
	}
	return nil
}
