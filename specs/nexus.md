# Nexus REST Client

## `pkg/nexus/`

- `Client` struct with URL, username, password
- `NewClient(url, username, password string) *Client`
- `RepoExists(ctx context.Context, name string) (bool, error)`
- `CreateRepo(ctx context.Context, name string, repoType string) error` — creates yum/apt/raw/nuget hosted repo
- `Upload(ctx context.Context, repoName, remotePath, localFilePath string) error`
- `UploadPackages(ctx context.Context, results []DownloadResult, repoPrefix string, createRepos bool) error` — orchestrates