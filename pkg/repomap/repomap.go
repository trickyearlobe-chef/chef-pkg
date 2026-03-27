package repomap

import (
	"fmt"
	"log"
)

// platformMap normalizes Chef API platform names to names used by package managers.
var platformMap = map[string]string{
	"amazon":   "amzn",
	"mac_os_x": "macos",
	"darwin":   "macos",
	"solaris2": "solaris",
}

// NormalizePlatform converts a Chef API platform name to its normalized form.
// Unknown platforms pass through unchanged.
func NormalizePlatform(chefPlatform string) string {
	if norm, ok := platformMap[chefPlatform]; ok {
		return norm
	}
	return chefPlatform
}

// ubuntuCodenames maps Ubuntu version numbers to release codenames.
var ubuntuCodenames = map[string]string{
	"14.04": "trusty",
	"16.04": "xenial",
	"18.04": "bionic",
	"20.04": "focal",
	"22.04": "jammy",
	"24.04": "noble",
}

// debianCodenames maps Debian version numbers to release codenames.
var debianCodenames = map[string]string{
	"8":  "jessie",
	"9":  "stretch",
	"10": "buster",
	"11": "bullseye",
	"12": "bookworm",
	"13": "trixie",
}

// NormalizePlatformVersion converts a Chef API platform version to the form
// used by the platform's native package manager. The literal string "pv"
// (returned by generic products like chef-360 and automate) is normalized to
// "generic" regardless of platform. For Ubuntu and Debian, numeric versions
// are converted to codenames. For all other platforms, the version is returned
// as-is. Unknown versions produce a warning and fall back to the raw version
// string.
func NormalizePlatformVersion(platform, version string) string {
	if version == "pv" {
		return "generic"
	}
	switch platform {
	case "ubuntu":
		if codename, ok := ubuntuCodenames[version]; ok {
			return codename
		}
		log.Printf("WARNING: unknown Ubuntu version %q, using raw version", version)
		return version
	case "debian":
		if codename, ok := debianCodenames[version]; ok {
			return codename
		}
		log.Printf("WARNING: unknown Debian version %q, using raw version", version)
		return version
	default:
		return version
	}
}

// aptArchMap normalizes Chef API architecture names to Debian/Ubuntu arch names.
var aptArchMap = map[string]string{
	"x86_64":  "amd64",
	"aarch64": "arm64",
	"ppc64le": "ppc64el",
}

// NormalizeArch converts a Chef API architecture name to the form used by the
// repo type's package manager. Only apt repos need normalization; all others
// use the Chef API name as-is.
func NormalizeArch(repoType, chefArch string) string {
	if repoType == "apt" {
		if norm, ok := aptArchMap[chefArch]; ok {
			return norm
		}
	}
	return chefArch
}

// yumPlatforms are platforms that use RPM/yum repositories.
var yumPlatforms = map[string]bool{
	"el":       true,
	"amazon":   true,
	"sles":     true,
	"opensuse": true,
	"rocky":    true,
	"alma":     true,
	"fedora":   true,
}

// aptPlatforms are platforms that use DEB/apt repositories.
var aptPlatforms = map[string]bool{
	"ubuntu": true,
	"debian": true,
}

// packageFormatRepoType maps package format strings (as found in the
// Architecture field for products like chef-ice) to artifact repository types.
var packageFormatRepoType = map[string]string{
	"deb": "apt",
	"rpm": "yum",
	"msi": "raw",
	"tar": "raw",
}

// IsPackageFormat returns true if the given string is a known package format
// rather than a CPU architecture. Products like chef-ice use the Architecture
// field to hold the package format (deb, rpm, tar, msi) instead of the CPU
// architecture (x86_64, aarch64, etc.).
func IsPackageFormat(arch string) bool {
	_, ok := packageFormatRepoType[arch]
	return ok
}

// RepoType returns the artifact repository type for a given Chef API platform.
// Returns "yum" for RPM-based, "apt" for DEB-based, and "raw" for everything else.
//
// For standard products (chef, inspec, etc.) the platform alone determines the
// repo type. For products like chef-ice where the Architecture field holds the
// package format, call RepoTypeFromPackageFormat instead.
func RepoType(platform string) string {
	if yumPlatforms[platform] {
		return "yum"
	}
	if aptPlatforms[platform] {
		return "apt"
	}
	return "raw"
}

// RepoTypeForPackage returns the artifact repository type by examining both
// the platform and architecture fields of a package. If the architecture
// field contains a package format (deb, rpm, tar, msi) — as used by products
// like chef-ice — the repo type is derived from that format. Otherwise, the
// repo type is derived from the platform in the standard way.
func RepoTypeForPackage(platform, arch string) string {
	if rt, ok := packageFormatRepoType[arch]; ok {
		return rt
	}
	return RepoType(platform)
}

// RepoName builds the artifact repository name from its components.
// Pattern: {prefix}-{normalizedPlatform}{normalizedVersion}-{repoType}
//
// Architecture is NOT included in the repo name because yum and apt repos
// natively support multiple architectures, and raw repos use path-based
// separation. All arches and all products coexist in one repo per
// platform+platformVersion.
//
// Platform and version normalization are applied automatically.
func RepoName(prefix, platform, platformVersion, repoType string) string {
	normPlatform := NormalizePlatform(platform)
	normVersion := NormalizePlatformVersion(platform, platformVersion)

	return fmt.Sprintf("%s-%s-%s-%s", prefix, normPlatform, normVersion, repoType)
}
