package artifactory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client talks to the JFrog Artifactory REST API.
type Client struct {
	BaseURL    string
	Token      string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// ClientOption is a functional option for configuring Client.
type ClientOption func(*Client)

// WithToken sets a Bearer token for authentication (takes precedence over basic auth).
func WithToken(token string) ClientOption {
	return func(c *Client) {
		c.Token = token
	}
}

// WithBasicAuth sets username/password for basic authentication.
func WithBasicAuth(username, password string) ClientOption {
	return func(c *Client) {
		c.Username = username
		c.Password = password
	}
}

// NewClient creates a new Artifactory client.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// doRequest executes an authenticated HTTP request.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("artifactory: creating request: %w", err)
	}

	// Token takes precedence over basic auth
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	} else if c.Username != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return c.HTTPClient.Do(req)
}

// RepoExists checks whether a repository exists in Artifactory.
func (c *Client) RepoExists(ctx context.Context, name string) (bool, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/repositories/"+name, nil, "")
	if err != nil {
		return false, fmt.Errorf("artifactory: checking repo %s: %w", name, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound, http.StatusBadRequest:
		return false, nil
	default:
		return false, fmt.Errorf("artifactory: checking repo %s: HTTP %d", name, resp.StatusCode)
	}
}

// packageType maps our internal repo types to Artifactory package types.
func packageType(repoType string) string {
	switch repoType {
	case "yum":
		return "yum"
	case "apt":
		return "debian"
	case "nuget":
		return "nuget"
	case "raw":
		return "generic"
	default:
		return "generic"
	}
}

// CreateRepo creates a local repository in Artifactory.
func (c *Client) CreateRepo(ctx context.Context, name, repoType string) error {
	payload := map[string]interface{}{
		"key":         name,
		"rclass":      "local",
		"packageType": packageType(repoType),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("artifactory: marshalling repo config: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPut, "/api/repositories/"+name, bytes.NewReader(payloadBytes), "application/json")
	if err != nil {
		return fmt.Errorf("artifactory: creating repo %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("artifactory: creating repo %s: HTTP %d: %s", name, resp.StatusCode, string(body))
	}
	return nil
}

// Upload deploys a local file to an Artifactory repository via PUT.
// remotePath is the path within the repository (e.g. "chef/18.4.12/chef_18.4.12-1_amd64.deb").
func (c *Client) Upload(ctx context.Context, repoName, remotePath, localFilePath string) error {
	f, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("artifactory: opening file %s: %w", localFilePath, err)
	}
	defer f.Close()

	path := fmt.Sprintf("/%s/%s", repoName, remotePath)
	resp, err := c.doRequest(ctx, http.MethodPut, path, f, "")
	if err != nil {
		return fmt.Errorf("artifactory: uploading to %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("artifactory: uploading to %s: HTTP %d: %s", path, resp.StatusCode, string(body))
	}
	return nil
}
