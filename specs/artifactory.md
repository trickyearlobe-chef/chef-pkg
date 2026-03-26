# Artifactory REST Client

## `pkg/artifactory/`

- `Client` struct with URL, token, username, password (token takes precedence)
- `NewClient(url string, opts ...ClientOption) *Client`
- `ClientOption`: `WithToken(token)`, `WithBasicAuth(username, password)`
- `RepoExists(ctx context.Context, name string) (bool, error)`
- `CreateRepo(ctx context.Context, name string, repoType string) error` — creates local yum/apt/generic/nuget repo
- `Upload(ctx context.Context, repoName, remotePath, localFilePath string) error` — `PUT` deploy
- `UploadPackages(ctx context.Context, results []DownloadResult, repoPrefix string, createRepos bool) error` — orchestrates