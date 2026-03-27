package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

// rawGetHandler returns an MCP tool handler for the raw_get tool.
// It captures the chefapi.Client so the handler is testable with httptest.
func rawGetHandler(client *chefapi.Client) func(ctx context.Context, req *sdkmcp.CallToolRequest, input RawGetInput) (*sdkmcp.CallToolResult, RawGetOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input RawGetInput) (*sdkmcp.CallToolResult, RawGetOutput, error) {
		if input.Path == "" {
			return nil, RawGetOutput{}, fmt.Errorf("path is required")
		}

		params := mapToURLValues(input.Params)

		body, err := client.RawGet(ctx, input.Path, params)
		if err != nil {
			// Check if it's an API error with a status code we can report
			if apiErr, ok := err.(*chefapi.APIError); ok {
				return nil, RawGetOutput{
					Path:       input.Path,
					StatusCode: apiErr.StatusCode,
					Body:       apiErr.Body,
				}, nil
			}
			return nil, RawGetOutput{}, fmt.Errorf("raw GET %s: %w", input.Path, err)
		}

		// Try to parse as JSON so the LLM gets structured data
		var parsed any
		if json.Unmarshal(body, &parsed) == nil {
			return nil, RawGetOutput{
				Path:       input.Path,
				StatusCode: 200,
				Body:       parsed,
			}, nil
		}

		// Fall back to raw string
		return nil, RawGetOutput{
			Path:       input.Path,
			StatusCode: 200,
			Body:       string(body),
		}, nil
	}
}

// listProductsHandler returns an MCP tool handler for the list_products tool.
// When include_eol is true, it fetches both current and eol product lists,
// then tags each product with its lifecycle status.
func listProductsHandler(client *chefapi.Client) func(ctx context.Context, req *sdkmcp.CallToolRequest, input ListProductsInput) (*sdkmcp.CallToolResult, ListProductsOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input ListProductsInput) (*sdkmcp.CallToolResult, ListProductsOutput, error) {
		// Fetch current products
		current, err := client.FetchProducts(ctx, nil)
		if err != nil {
			return nil, ListProductsOutput{}, fmt.Errorf("list products: %w", err)
		}

		if !input.IncludeEOL {
			products := make([]ProductInfo, len(current))
			for i, name := range current {
				products[i] = ProductInfo{Name: name, Status: "current"}
			}
			return nil, ListProductsOutput{Products: products}, nil
		}

		// Fetch all products including eol
		allProducts, err := client.FetchProducts(ctx, url.Values{"eol": {"true"}})
		if err != nil {
			return nil, ListProductsOutput{}, fmt.Errorf("list products (eol): %w", err)
		}

		// Build a set of current product names for fast lookup
		currentSet := make(map[string]bool, len(current))
		for _, name := range current {
			currentSet[name] = true
		}

		products := make([]ProductInfo, len(allProducts))
		for i, name := range allProducts {
			status := "eol"
			if currentSet[name] {
				status = "current"
			}
			products[i] = ProductInfo{Name: name, Status: status}
		}

		return nil, ListProductsOutput{Products: products}, nil
	}
}

// listVersionsHandler returns an MCP tool handler for the list_versions tool.
// It captures the default channel so omitted input fields fall back to config.
func listVersionsHandler(client *chefapi.Client, defaultChannel string) func(ctx context.Context, req *sdkmcp.CallToolRequest, input ListVersionsInput) (*sdkmcp.CallToolResult, ListVersionsOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input ListVersionsInput) (*sdkmcp.CallToolResult, ListVersionsOutput, error) {
		product := input.Product
		if product == "" {
			product = "chef"
		}
		channel := input.Channel
		if channel == "" {
			channel = defaultChannel
		}

		versions, err := client.FetchVersions(ctx, channel, product)
		if err != nil {
			return nil, ListVersionsOutput{}, fmt.Errorf("list versions %s/%s: %w", channel, product, err)
		}

		return nil, ListVersionsOutput{
			Product:  product,
			Channel:  channel,
			Versions: versions,
		}, nil
	}
}

// listPackagesHandler returns an MCP tool handler for the list_packages tool.
// It fetches packages, flattens the nested response, and applies optional
// platform/arch substring filters (case-insensitive).
func listPackagesHandler(client *chefapi.Client, defaultChannel string) func(ctx context.Context, req *sdkmcp.CallToolRequest, input ListPackagesInput) (*sdkmcp.CallToolResult, ListPackagesOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input ListPackagesInput) (*sdkmcp.CallToolResult, ListPackagesOutput, error) {
		product := input.Product
		if product == "" {
			product = "chef"
		}
		version := input.Version
		if version == "" {
			version = "latest"
		}
		channel := input.Channel
		if channel == "" {
			channel = defaultChannel
		}

		pkgResp, err := client.FetchPackages(ctx, channel, product, version)
		if err != nil {
			return nil, ListPackagesOutput{}, fmt.Errorf("list packages %s/%s@%s: %w", channel, product, version, err)
		}

		flat := pkgResp.Flatten()

		// Apply optional filters
		platformFilter := strings.ToLower(input.Platform)
		archFilter := strings.ToLower(input.Arch)

		var packages []PackageInfo
		for _, fp := range flat {
			if platformFilter != "" && !strings.Contains(strings.ToLower(fp.Platform), platformFilter) {
				continue
			}
			if archFilter != "" && !strings.Contains(strings.ToLower(fp.Architecture), archFilter) {
				continue
			}
			packages = append(packages, PackageInfo{
				Platform:        fp.Platform,
				PlatformVersion: fp.PlatformVersion,
				Architecture:    fp.Architecture,
				Version:         fp.Version,
				SHA256:          fp.SHA256,
			})
		}

		return nil, ListPackagesOutput{
			Product:  product,
			Version:  version,
			Channel:  channel,
			Packages: packages,
		}, nil
	}
}

// mapToURLValues converts a map[string]string to url.Values.
func mapToURLValues(m map[string]string) url.Values {
	if len(m) == 0 {
		return nil
	}
	vals := make(url.Values, len(m))
	for k, v := range m {
		vals.Set(k, v)
	}
	return vals
}
