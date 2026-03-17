package chefapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// FetchPackages retrieves the package list for a given channel, product, and version.
func (c *Client) FetchPackages(ctx context.Context, channel, product, version string) (PackagesResponse, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s/packages", c.BaseURL, channel, product))
	if err != nil {
		return nil, fmt.Errorf("chefapi: building URL: %w", err)
	}

	q := u.Query()
	q.Set("v", version)
	q.Set("license_id", c.LicenseID)
	u.RawQuery = q.Encode()

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

	var result PackagesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("chefapi: decoding response: %w", err)
	}

	return result, nil
}
