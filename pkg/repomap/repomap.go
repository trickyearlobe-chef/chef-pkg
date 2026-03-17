package repomap

import (
	"fmt"
	"log"
)

// platformMap normalizes Chef API platform names to names used by package managers.
var platformMap = map[string]string{
	"amazon":   "amzn",
	"mac_os_x": "macos",
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
// used by the platform's native package manager. For Ubuntu and Debian, this
// means converting numeric versions to codenames. For all other platforms,
// the version is returned as-is. Unknown versions produce a warning and fall
// back to the raw version string.
func NormalizePlatformVersion(platform, version string) string {
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

// RepoType returns the artifact repository type for a given Chef API platform.
// Returns "yum" for RPM-based, "apt" for DEB-based, and "raw" for everything else.
func RepoType(platform string) string {
	if yumPlatforms[platform] {
		return "yum"
	}
	if aptPlatforms[platform] {
		return "apt"
	}
	return "raw"
}

// RepoName builds the artifact repository name from its components.
// Pattern: {prefix}-{normalizedPlatform}{normalizedVersion}-{normalizedArch}-{repoType}
//
// Platform and version normalization are applied automatically.
// Architecture is normalized based on the repo type.
func RepoName(prefix, platform, platformVersion, arch, repoType string) string {
	normPlatform := NormalizePlatform(platform)
	normVersion := NormalizePlatformVersion(platform, platformVersion)
	normArch := NormalizeArch(repoType, arch)

	// For apt, use a hyphen between platform and codename for readability
	// e.g. chef-ubuntu-jammy-amd64-apt rather than chef-ubuntujammy-amd64-apt
	if repoType == "apt" {
		return fmt.Sprintf("%s-%s-%s-%s-%s", prefix, normPlatform, normVersion, normArch, repoType)
	}
	return fmt.Sprintf("%s-%s%s-%s-%s", prefix, normPlatform, normVersion, normArch, repoType)
}
