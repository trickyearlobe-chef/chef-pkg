package chefapi

import (
	"fmt"
	"sort"
)

// PackageDetail holds info about a single downloadable package artifact.
type PackageDetail struct {
	SHA1    string `json:"sha1"`
	SHA256  string `json:"sha256"`
	URL     string `json:"url"`
	Version string `json:"version"`
}

// PackagesResponse represents the full nested API response:
// platform -> platform_version -> architecture -> PackageDetail
type PackagesResponse map[string]map[string]map[string]PackageDetail

// FlatPackage is a denormalized view of a single package with its
// platform, version, and architecture info included.
type FlatPackage struct {
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	Architecture    string `json:"architecture"`
	PackageDetail
}

// Flatten converts the nested PackagesResponse into a sorted slice of FlatPackage.
// Results are sorted by Platform, then PlatformVersion, then Architecture.
func (r PackagesResponse) Flatten() []FlatPackage {
	var packages []FlatPackage
	for platform, versions := range r {
		for version, archs := range versions {
			for arch, detail := range archs {
				packages = append(packages, FlatPackage{
					Platform:        platform,
					PlatformVersion: version,
					Architecture:    arch,
					PackageDetail:   detail,
				})
			}
		}
	}
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Platform != packages[j].Platform {
			return packages[i].Platform < packages[j].Platform
		}
		if packages[i].PlatformVersion != packages[j].PlatformVersion {
			return packages[i].PlatformVersion < packages[j].PlatformVersion
		}
		return packages[i].Architecture < packages[j].Architecture
	})
	return packages
}

// APIError represents an error response from the Chef downloads API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("chefapi: HTTP %d: %s", e.StatusCode, e.Body)
}
