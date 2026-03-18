package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

// parseSemver extracts major, minor, patch from a version string.
// It handles versions with or without a "v" prefix and ignores
// pre-release/build metadata for sorting purposes.
// Returns (0,0,0, false) if the string is not a valid semver.
func parseSemver(version string) (major, minor, patch int, ok bool) {
	v := strings.TrimPrefix(version, "v")

	// Strip pre-release and build metadata for comparison
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return 0, 0, 0, false
	}

	nums := make([]int, 3)
	for i, p := range parts {
		n := 0
		if p == "" {
			return 0, 0, 0, false
		}
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return 0, 0, 0, false
			}
			n = n*10 + int(ch-'0')
		}
		nums[i] = n
	}

	return nums[0], nums[1], nums[2], true
}

// semverLess returns true if version a is less than version b.
func semverLess(a, b string) bool {
	aMaj, aMin, aPat, aOk := parseSemver(a)
	bMaj, bMin, bPat, bOk := parseSemver(b)

	// Non-semver strings sort before valid semver
	if !aOk && !bOk {
		return a < b
	}
	if !aOk {
		return true
	}
	if !bOk {
		return false
	}

	if aMaj != bMaj {
		return aMaj < bMaj
	}
	if aMin != bMin {
		return aMin < bMin
	}
	return aPat < bPat
}

// sortVersionsSemver sorts a slice of version strings in ascending semver order.
func sortVersionsSemver(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		return semverLess(versions[i], versions[j])
	})
}

// resolveVersions interprets the --version flag value and returns a list of
// concrete version strings to operate on. It supports:
//
//   - An exact version string (e.g. "18.4.12") — returned as-is.
//   - "latest" — resolves to the most recent version that has packages
//     matching the given platform/arch filters. This requires API calls.
//   - "all" — returns every available version in ascending semver order.
//
// The client, channel, and product are used to query the API when needed.
// The platform and arch filters are only relevant for "latest" resolution,
// where we need to find the newest version that actually has packages for
// the requested platform/architecture combination.
func resolveVersions(
	ctx context.Context,
	client *chefapi.Client,
	channel, product, versionFlag, platform, arch string,
) ([]string, error) {
	switch strings.ToLower(versionFlag) {
	case "latest":
		return resolveLatest(ctx, client, channel, product, platform, arch)
	case "all":
		return resolveAll(ctx, client, channel, product)
	default:
		major, minor, patch, ok := parseSemver(versionFlag)
		if !ok {
			return nil, fmt.Errorf("invalid version %q: expected semver (e.g. 18.4.12), 'latest', or 'all'", versionFlag)
		}
		if minor == 0 && patch == 0 && !strings.Contains(versionFlag, ".") {
			return resolveMajor(ctx, client, channel, product, major, platform, arch)
		}
		return []string{versionFlag}, nil
	}
}

// resolveAll fetches all available versions and returns them sorted in
// ascending semver order.
func resolveAll(
	ctx context.Context,
	client *chefapi.Client,
	channel, product string,
) ([]string, error) {
	fmt.Fprintf(os.Stderr, "Fetching all versions for %s (%s channel)...\n", product, channel)

	versions, err := client.FetchVersions(ctx, channel, product)
	if err != nil {
		return nil, fmt.Errorf("fetching versions: %w", err)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for %s on %s channel", product, channel)
	}

	sortVersionsSemver(versions)
	fmt.Fprintf(os.Stderr, "Found %d version(s)\n", len(versions))
	return versions, nil
}

// resolveLatest finds the most recent version that has packages matching the
// given platform/arch filters.
//
// When no platform or arch filter is specified, it simply returns the highest
// semver version (the API guarantees at least some packages for listed versions).
//
// When a platform or arch filter IS specified, we walk versions from newest to
// oldest and fetch the package list for each until we find one that has matching
// packages. This avoids returning a "latest" version that doesn't have packages
// for the requested platform (e.g. the newest version might drop Solaris support).
func resolveLatest(
	ctx context.Context,
	client *chefapi.Client,
	channel, product, platform, arch string,
) ([]string, error) {
	fmt.Fprintf(os.Stderr, "Resolving latest version for %s (%s channel)", product, channel)
	if platform != "" || arch != "" {
		filters := []string{}
		if platform != "" {
			filters = append(filters, "platform="+platform)
		}
		if arch != "" {
			filters = append(filters, "arch="+arch)
		}
		fmt.Fprintf(os.Stderr, " [%s]", strings.Join(filters, ", "))
	}
	fmt.Fprintln(os.Stderr, "...")

	versions, err := client.FetchVersions(ctx, channel, product)
	if err != nil {
		return nil, fmt.Errorf("fetching versions: %w", err)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for %s on %s channel", product, channel)
	}

	sortVersionsSemver(versions)

	// If no platform/arch filter, the highest version is the answer
	if platform == "" && arch == "" {
		latest := versions[len(versions)-1]
		fmt.Fprintf(os.Stderr, "Resolved latest version: %s\n", latest)
		return []string{latest}, nil
	}

	// Walk from newest to oldest, find the first version with matching packages
	for i := len(versions) - 1; i >= 0; i-- {
		v := versions[i]
		resp, err := client.FetchPackages(ctx, channel, product, v)
		if err != nil {
			// Skip versions that error (might be unlisted/broken)
			fmt.Fprintf(os.Stderr, "  Skipping %s (error: %v)\n", v, err)
			continue
		}

		packages := resp.Flatten()
		packages = filterPackages(packages, platform, arch)

		if len(packages) > 0 {
			fmt.Fprintf(os.Stderr, "Resolved latest version: %s (%d matching package(s))\n", v, len(packages))
			return []string{v}, nil
		}
	}

	return nil, fmt.Errorf("no version of %s on %s channel has packages matching platform=%q arch=%q",
		product, channel, platform, arch)
}

// resolveMajor returns all versions within a major release line, newest last.
func resolveMajor(
	ctx context.Context,
	client *chefapi.Client,
	channel, product string,
	major int,
	platform, arch string,
) ([]string, error) {
	versions, err := client.FetchVersions(ctx, channel, product)
	if err != nil {
		return nil, fmt.Errorf("fetching versions: %w", err)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for %s on %s channel", product, channel)
	}

	var matching []string
	for _, v := range versions {
		vMajor, _, _, ok := parseSemver(v)
		if ok && vMajor == major {
			matching = append(matching, v)
		}
	}

	if len(matching) == 0 {
		if platform == "" && arch == "" {
			if resp, err := client.FetchPackages(ctx, channel, product, fmt.Sprintf("%d", major)); err == nil {
				if packages := resp.Flatten(); len(packages) > 0 {
					return []string{fmt.Sprintf("%d", major)}, nil
				}
			}
		}
		return nil, fmt.Errorf("no version of %s on %s channel matches major=%d platform=%q arch=%q",
			product, channel, major, platform, arch)
	}

	sortVersionsSemver(matching)
	if platform == "" && arch == "" {
		return matching, nil
	}

	var filtered []string
	for _, v := range matching {
		resp, err := client.FetchPackages(ctx, channel, product, v)
		if err != nil {
			continue
		}
		packages := filterPackages(resp.Flatten(), platform, arch)
		if len(packages) > 0 {
			filtered = append(filtered, v)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no version of %s on %s channel matches major=%d platform=%q arch=%q",
			product, channel, major, platform, arch)
	}
	return filtered, nil
}
