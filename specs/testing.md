# Testing Strategy

- `httptest`-based tests for `pkg/chefapi/` client
- Test scenarios: success, HTTP error (403, 404, 500), invalid JSON
- Tests for `Flatten()` method correctness and sort order
- Tests for `pkg/repomap/` normalization functions
- Tests for `pkg/downloader/` with mock HTTP server
- Tests for `pkg/nexus/` and `pkg/artifactory/` with mock HTTP servers