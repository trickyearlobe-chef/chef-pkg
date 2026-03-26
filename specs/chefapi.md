# Chef Downloads API

## API Details

- **Base URL**: `https://commercial-acceptance.downloads.chef.co`
- **Path pattern**: `/{channel}/{product}/packages`
- **Query params**: `v` (version), `license_id` (required)
- **Response format (packages)**: Nested JSON map — `platform → platform_version → architecture → PackageDetail`
- **Products endpoint**: `GET /products?license_id={id}` — returns `["automate", "chef", ...]`
- **Versions endpoint**: `GET /{channel}/{product}/versions/all?license_id={id}` — returns `["18.4.12", ...]`

## Package Design — `pkg/chefapi/`

- `Client` struct with functional options pattern
- `ClientOption` type: `WithBaseURL(url)`, `WithHTTPClient(c)`
- `NewClient(licenseID string, opts ...ClientOption) *Client`
- `FetchProducts(ctx context.Context) ([]string, error)` — calls `/products`
- `FetchVersions(ctx context.Context, channel, product string) ([]string, error)` — calls `/{channel}/{product}/versions/all`
- `FetchPackages(ctx context.Context, channel, product, version string) (PackagesResponse, error)`
- `PackagesResponse` = `map[string]map[string]map[string]PackageDetail`
- `PackageDetail` struct: `SHA1`, `SHA256`, `URL`, `Version`
- `FlatPackage` struct: `Platform`, `PlatformVersion`, `Architecture` + embedded `PackageDetail`
- `Flatten() → []FlatPackage` — sorted by Platform, PlatformVersion, Architecture
- `APIError` custom error type with status code and response body