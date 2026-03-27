package mcp

// RawGetInput is the input schema for the raw_get tool.
type RawGetInput struct {
	Path   string            `json:"path" jsonschema:"API path (e.g. /stable/chef/versions/all)"`
	Params map[string]string `json:"params,omitempty" jsonschema:"Additional query parameters. license_id is added automatically."`
}

// RawGetOutput is the output schema for the raw_get tool.
type RawGetOutput struct {
	Path       string `json:"path" jsonschema:"The API path that was requested"`
	StatusCode int    `json:"status_code" jsonschema:"HTTP status code from the API"`
	Body       any    `json:"body" jsonschema:"Parsed JSON response body, or raw string if not valid JSON"`
}

// ListProductsInput is the input schema for the list_products tool.
type ListProductsInput struct {
	IncludeEOL bool `json:"include_eol,omitempty" jsonschema:"Include end-of-life products (default false)"`
}

// ProductInfo describes a single Chef product with its lifecycle status.
type ProductInfo struct {
	Name   string `json:"name" jsonschema:"Product name (e.g. chef, inspec, automate)"`
	Status string `json:"status" jsonschema:"Lifecycle status: current or eol"`
}

// ListProductsOutput is the output schema for the list_products tool.
type ListProductsOutput struct {
	Products []ProductInfo `json:"products" jsonschema:"List of available Chef products"`
}

// ListVersionsInput is the input schema for the list_versions tool.
type ListVersionsInput struct {
	Product string `json:"product,omitempty" jsonschema:"Product name (default chef)"`
	Channel string `json:"channel,omitempty" jsonschema:"Release channel (default stable)"`
}

// ListVersionsOutput is the output schema for the list_versions tool.
type ListVersionsOutput struct {
	Product  string   `json:"product" jsonschema:"Product that was queried"`
	Channel  string   `json:"channel" jsonschema:"Release channel that was queried"`
	Versions []string `json:"versions" jsonschema:"Available versions in ascending order"`
}

// ListPackagesInput is the input schema for the list_packages tool.
type ListPackagesInput struct {
	Product  string `json:"product,omitempty" jsonschema:"Product name (default chef)"`
	Version  string `json:"version,omitempty" jsonschema:"Version: semver or latest (default latest)"`
	Channel  string `json:"channel,omitempty" jsonschema:"Release channel (default stable)"`
	Platform string `json:"platform,omitempty" jsonschema:"Filter by platform (case-insensitive substring match)"`
	Arch     string `json:"arch,omitempty" jsonschema:"Filter by architecture (case-insensitive substring match)"`
}

// PackageInfo describes a single downloadable package artifact.
// The URL field is intentionally omitted because it embeds the license_id.
type PackageInfo struct {
	Platform        string `json:"platform" jsonschema:"Operating system (e.g. ubuntu, windows, el)"`
	PlatformVersion string `json:"platform_version" jsonschema:"OS version (e.g. 22.04, 9, 2019)"`
	Architecture    string `json:"architecture" jsonschema:"CPU architecture (e.g. x86_64, aarch64)"`
	Version         string `json:"version" jsonschema:"Package version"`
	SHA256          string `json:"sha256" jsonschema:"SHA-256 checksum of the package file"`
}

// ListPackagesOutput is the output schema for the list_packages tool.
type ListPackagesOutput struct {
	Product  string        `json:"product" jsonschema:"Product that was queried"`
	Version  string        `json:"version" jsonschema:"Version that was queried"`
	Channel  string        `json:"channel" jsonschema:"Release channel that was queried"`
	Packages []PackageInfo `json:"packages" jsonschema:"Available packages matching the filters"`
}
