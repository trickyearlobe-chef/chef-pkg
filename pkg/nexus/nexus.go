package nexus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client talks to the Sonatype Nexus Repository Manager REST API.
type Client struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// NewClient creates a new Nexus client.
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		BaseURL:    baseURL,
		Username:   username,
		Password:   password,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// doRequest executes an authenticated HTTP request.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("nexus: creating request: %w", err)
	}
	req.SetBasicAuth(c.Username, c.Password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.HTTPClient.Do(req)
}

// RepoExists checks whether a repository exists in Nexus.
func (c *Client) RepoExists(ctx context.Context, name string) (bool, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/service/rest/v1/repositories/"+name, nil)
	if err != nil {
		return false, fmt.Errorf("nexus: checking repo %s: %w", name, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("nexus: checking repo %s: HTTP %d", name, resp.StatusCode)
	}
}

// Repos lists the repository names visible to the Nexus instance.
func (c *Client) Repos(ctx context.Context) ([]string, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/service/rest/v1/repositories", nil)
	if err != nil {
		return nil, fmt.Errorf("nexus: listing repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nexus: listing repositories: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("nexus: decoding repositories: %w", err)
	}

	names := make([]string, 0, len(payload))
	for _, repo := range payload {
		if repo.Name != "" {
			names = append(names, repo.Name)
		}
	}
	return names, nil
}

// DeleteRepo deletes a repository by name.
func (c *Client) DeleteRepo(ctx context.Context, name string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/service/rest/v1/repositories/"+name, nil)
	if err != nil {
		return fmt.Errorf("nexus: deleting repo %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("nexus: deleting repo %s: HTTP %d: %s", name, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// CreateRepoOption is a functional option for configuring repository creation.
type CreateRepoOption func(*createRepoConfig)

// createRepoConfig holds optional configuration for CreateRepo.
type createRepoConfig struct {
	gpgKeypair    string
	gpgPassphrase string
}

// WithGPGKeypair sets the GPG keypair name for APT repository signing.
// This is required by Nexus when creating APT hosted repositories.
// The keypair must already be stored in Nexus (via the GPG keys admin UI).
func WithGPGKeypair(keypair string) CreateRepoOption {
	return func(cfg *createRepoConfig) {
		cfg.gpgKeypair = keypair
	}
}

// WithGPGPassphrase sets the passphrase for the GPG keypair used for
// APT repository signing. If the keypair has no passphrase, this can
// be omitted.
func WithGPGPassphrase(passphrase string) CreateRepoOption {
	return func(cfg *createRepoConfig) {
		cfg.gpgPassphrase = passphrase
	}
}

// repoPayload builds the JSON payload for creating a hosted repository.
func repoPayload(name, repoType string, cfg *createRepoConfig) ([]byte, error) {
	var payload map[string]interface{}

	switch repoType {
	case "yum":
		payload = map[string]interface{}{
			"name":   name,
			"online": true,
			"storage": map[string]interface{}{
				"blobStoreName":               "default",
				"strictContentTypeValidation": true,
				"writePolicy":                 "ALLOW",
			},
			"yum": map[string]interface{}{
				"repodataDepth": 0,
				"deployPolicy":  "STRICT",
			},
		}
	case "apt":
		if cfg.gpgKeypair == "" {
			return nil, fmt.Errorf("nexus: APT repositories require a GPG keypair for signing; set --nexus-gpg-keypair, config nexus.gpg_keypair, or CHEFPKG_NEXUS_GPG_KEYPAIR")
		}
		aptSigning := map[string]interface{}{
			"keypair": cfg.gpgKeypair,
		}
		if cfg.gpgPassphrase != "" {
			aptSigning["passphrase"] = cfg.gpgPassphrase
		}
		payload = map[string]interface{}{
			"name":   name,
			"online": true,
			"storage": map[string]interface{}{
				"blobStoreName":               "default",
				"strictContentTypeValidation": true,
				"writePolicy":                 "ALLOW",
			},
			"apt": map[string]interface{}{
				"distribution": "stable",
			},
			"aptSigning": aptSigning,
		}
	case "nuget":
		payload = map[string]interface{}{
			"name":   name,
			"online": true,
			"storage": map[string]interface{}{
				"blobStoreName":               "default",
				"strictContentTypeValidation": true,
				"writePolicy":                 "ALLOW",
			},
		}
	case "raw":
		payload = map[string]interface{}{
			"name":   name,
			"online": true,
			"storage": map[string]interface{}{
				"blobStoreName":               "default",
				"strictContentTypeValidation": false,
				"writePolicy":                 "ALLOW",
			},
		}
	default:
		return nil, fmt.Errorf("nexus: unsupported repo type: %s", repoType)
	}

	return json.Marshal(payload)
}

// CreateRepo creates a hosted repository in Nexus.
// For APT repositories, use WithGPGKeypair to provide the required signing key.
func (c *Client) CreateRepo(ctx context.Context, name, repoType string, opts ...CreateRepoOption) error {
	cfg := &createRepoConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	payloadBytes, err := repoPayload(name, repoType, cfg)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/service/rest/v1/repositories/%s/hosted", repoType)
	resp, err := c.doRequest(ctx, http.MethodPost, path, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("nexus: creating repo %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("nexus: creating repo %s: HTTP %d: %s", name, resp.StatusCode, string(body))
	}
	return nil
}

// Upload uploads a local file to a Nexus repository.
// remotePath is the path within the repository (e.g. "chef/18.4.12/chef_18.4.12-1_amd64.deb").
func (c *Client) Upload(ctx context.Context, repoName, remotePath, localFilePath string) error {
	f, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("nexus: opening file %s: %w", localFilePath, err)
	}
	defer f.Close()

	path := fmt.Sprintf("/repository/%s/%s", repoName, remotePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.BaseURL+path, f)
	if err != nil {
		return fmt.Errorf("nexus: creating upload request: %w", err)
	}
	req.SetBasicAuth(c.Username, c.Password)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("nexus: uploading to %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("nexus: uploading to %s: HTTP %d: %s", path, resp.StatusCode, string(body))
	}
	return nil
}
