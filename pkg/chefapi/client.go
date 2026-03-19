package chefapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://commercial-acceptance.downloads.chef.co"

// Client talks to the Chef commercial downloads API.
type Client struct {
	BaseURL    string
	LicenseID  string
	HTTPClient *http.Client
}

// ClientOption is a functional option for configuring Client.
type ClientOption func(*Client)

// WithBaseURL overrides the default base URL.
func WithBaseURL(u string) ClientOption {
	return func(c *Client) {
		c.BaseURL = u
	}
}

// WithHTTPClient overrides the default http.Client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.HTTPClient = hc
	}
}

// NewClient creates a new Client. licenseID is required.
func NewClient(licenseID string, opts ...ClientOption) *Client {
	c := &Client{
		BaseURL:    defaultBaseURL,
		LicenseID:  licenseID,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// doGet performs a GET request and returns the response body bytes.
// It returns an APIError for non-200 status codes.
func (c *Client) doGet(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("chefapi: building URL: %w", err)
	}

	if params == nil {
		params = url.Values{}
	}
	params.Set("license_id", c.LicenseID)
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("chefapi: creating request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chefapi: executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("chefapi: reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	return body, nil
}

// RawGet performs a GET request against an arbitrary API path.
func (c *Client) RawGet(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.doGet(ctx, path, params)
}

// FetchProducts retrieves the list of available products.
// If params is non-nil, those query parameters are forwarded to the API
// (e.g. url.Values{"obsolete": {"true"}} to include end-of-life products).
func (c *Client) FetchProducts(ctx context.Context, params url.Values) ([]string, error) {
	body, err := c.doGet(ctx, "/products", params)
	if err != nil {
		return nil, err
	}

	var products []string
	if err := json.Unmarshal(body, &products); err != nil {
		return nil, fmt.Errorf("chefapi: decoding products: %w", err)
	}
	return products, nil
}

// FetchVersions retrieves the list of available versions for a product and channel.
func (c *Client) FetchVersions(ctx context.Context, channel, product string) ([]string, error) {
	endpoint := fmt.Sprintf("/%s/%s/versions/all", channel, product)
	body, err := c.doGet(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var versions []string
	if err := json.Unmarshal(body, &versions); err != nil {
		return nil, fmt.Errorf("chefapi: decoding versions: %w", err)
	}
	return versions, nil
}

// FetchPackages retrieves the package list for a given channel, product, and version.
func (c *Client) FetchPackages(ctx context.Context, channel, product, version string) (PackagesResponse, error) {
	endpoint := fmt.Sprintf("/%s/%s/packages", channel, product)
	params := url.Values{}
	params.Set("v", version)

	body, err := c.doGet(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var result PackagesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("chefapi: decoding packages: %w", err)
	}
	return result, nil
}
