package repomap

import (
	"testing"
)

func TestNormalizePlatform(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"el", "el"},
		{"amazon", "amzn"},
		{"sles", "sles"},
		{"opensuse", "opensuse"},
		{"ubuntu", "ubuntu"},
		{"debian", "debian"},
		{"rocky", "rocky"},
		{"alma", "alma"},
		{"fedora", "fedora"},
		{"windows", "windows"},
		{"mac_os_x", "macos"},
		{"darwin", "macos"},
		{"solaris2", "solaris"},
		{"freebsd", "freebsd"},
		{"linux-kernel2", "linux-kernel2"},
		{"aix", "aix"},
		// Unknown platforms pass through unchanged
		{"gentoo", "gentoo"},
	}
	for _, tt := range tests {
		got := NormalizePlatform(tt.input)
		if got != tt.want {
			t.Errorf("NormalizePlatform(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizePlatformVersion(t *testing.T) {
	tests := []struct {
		platform string
		version  string
		want     string
	}{
		// "pv" normalizes to "generic" for all platforms (before codename lookups)
		{"linux", "pv", "generic"},
		{"darwin", "pv", "generic"},
		{"windows", "pv", "generic"},
		{"ubuntu", "pv", "generic"},
		{"el", "pv", "generic"},
		// Ubuntu codenames
		{"ubuntu", "18.04", "bionic"},
		{"ubuntu", "20.04", "focal"},
		{"ubuntu", "22.04", "jammy"},
		{"ubuntu", "24.04", "noble"},
		// Debian codenames
		{"debian", "9", "stretch"},
		{"debian", "10", "buster"},
		{"debian", "11", "bullseye"},
		{"debian", "12", "bookworm"},
		// Non-apt platforms pass through
		{"el", "9", "9"},
		{"amazon", "2023", "2023"},
		{"windows", "2019", "2019"},
		{"sles", "15", "15"},
		{"mac_os_x", "12", "12"},
		// Unknown Ubuntu version falls back to raw
		{"ubuntu", "99.04", "99.04"},
		// Unknown Debian version falls back to raw
		{"debian", "99", "99"},
	}
	for _, tt := range tests {
		got := NormalizePlatformVersion(tt.platform, tt.version)
		if got != tt.want {
			t.Errorf("NormalizePlatformVersion(%q, %q) = %q, want %q", tt.platform, tt.version, got, tt.want)
		}
	}
}

func TestNormalizeArch(t *testing.T) {
	tests := []struct {
		repoType string
		arch     string
		want     string
	}{
		// apt normalizations
		{"apt", "x86_64", "amd64"},
		{"apt", "aarch64", "arm64"},
		{"apt", "ppc64le", "ppc64el"},
		{"apt", "i386", "i386"},
		{"apt", "s390x", "s390x"},
		// yum keeps as-is
		{"yum", "x86_64", "x86_64"},
		{"yum", "aarch64", "aarch64"},
		{"yum", "ppc64le", "ppc64le"},
		{"yum", "s390x", "s390x"},
		// raw keeps as-is
		{"raw", "x86_64", "x86_64"},
		{"raw", "aarch64", "aarch64"},
		{"raw", "powerpc", "powerpc"},
		{"raw", "sparc", "sparc"},
		{"raw", "i386", "i386"},
		// nuget keeps as-is
		{"nuget", "x86_64", "x86_64"},
		// Unknown arch passes through
		{"apt", "riscv64", "riscv64"},
	}
	for _, tt := range tests {
		got := NormalizeArch(tt.repoType, tt.arch)
		if got != tt.want {
			t.Errorf("NormalizeArch(%q, %q) = %q, want %q", tt.repoType, tt.arch, got, tt.want)
		}
	}
}

func TestRepoType(t *testing.T) {
	tests := []struct {
		platform string
		want     string
	}{
		// RPM-based
		{"el", "yum"},
		{"amazon", "yum"},
		{"sles", "yum"},
		{"opensuse", "yum"},
		{"rocky", "yum"},
		{"alma", "yum"},
		{"fedora", "yum"},
		// DEB-based
		{"ubuntu", "apt"},
		{"debian", "apt"},
		// Everything else is raw
		{"windows", "raw"},
		{"mac_os_x", "raw"},
		{"solaris2", "raw"},
		{"freebsd", "raw"},
		{"aix", "raw"},
		{"unknown", "raw"},
	}
	for _, tt := range tests {
		got := RepoType(tt.platform)
		if got != tt.want {
			t.Errorf("RepoType(%q) = %q, want %q", tt.platform, got, tt.want)
		}
	}
}

func TestIsPackageFormat(t *testing.T) {
	tests := []struct {
		arch string
		want bool
	}{
		// Known package formats
		{"deb", true},
		{"rpm", true},
		{"msi", true},
		{"tar", true},
		// CPU architectures are not package formats
		{"x86_64", false},
		{"aarch64", false},
		{"ppc64le", false},
		{"s390x", false},
		{"sparc", false},
		{"powerpc", false},
		{"i386", false},
		// Unknown strings are not package formats
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsPackageFormat(tt.arch)
		if got != tt.want {
			t.Errorf("IsPackageFormat(%q) = %v, want %v", tt.arch, got, tt.want)
		}
	}
}

func TestRepoTypeForPackage(t *testing.T) {
	tests := []struct {
		platform string
		arch     string
		want     string
	}{
		// Standard products: repo type from platform, arch is CPU arch
		{"el", "x86_64", "yum"},
		{"amazon", "aarch64", "yum"},
		{"ubuntu", "x86_64", "apt"},
		{"debian", "aarch64", "apt"},
		{"windows", "x86_64", "raw"},
		{"mac_os_x", "x86_64", "raw"},
		// chef-ice style: arch is package format, platform is generic
		{"linux", "rpm", "yum"},
		{"linux", "deb", "apt"},
		{"linux", "tar", "raw"},
		{"windows", "msi", "raw"},
		{"windows", "tar", "raw"},
		// Package format takes precedence over platform
		{"el", "deb", "apt"},
		{"ubuntu", "rpm", "yum"},
	}
	for _, tt := range tests {
		got := RepoTypeForPackage(tt.platform, tt.arch)
		if got != tt.want {
			t.Errorf("RepoTypeForPackage(%q, %q) = %q, want %q", tt.platform, tt.arch, got, tt.want)
		}
	}
}

func TestRepoName(t *testing.T) {
	tests := []struct {
		prefix          string
		platform        string
		platformVersion string
		repoType        string
		want            string
	}{
		// Arch is no longer in repo name — repos hold all arches
		// All repos use hyphen separator between platform and version
		{"chef", "el", "9", "yum", "chef-el-9-yum"},
		{"chef", "el", "8", "yum", "chef-el-8-yum"},
		{"chef", "amazon", "2023", "yum", "chef-amzn-2023-yum"},
		{"chef", "amazon", "2", "yum", "chef-amzn-2-yum"},
		{"chef", "sles", "15", "yum", "chef-sles-15-yum"},
		// APT repos use hyphen separator for readability
		{"chef", "ubuntu", "22.04", "apt", "chef-ubuntu-jammy-apt"},
		{"chef", "ubuntu", "24.04", "apt", "chef-ubuntu-noble-apt"},
		{"chef", "debian", "12", "apt", "chef-debian-bookworm-apt"},
		{"chef", "debian", "11", "apt", "chef-debian-bullseye-apt"},
		// Raw repos
		{"chef", "windows", "2019", "raw", "chef-windows-2019-raw"},
		{"chef", "mac_os_x", "12", "raw", "chef-macos-12-raw"},
		{"chef", "darwin", "pv", "raw", "chef-macos-generic-raw"},
		{"chef", "solaris2", "5.11", "raw", "chef-solaris-5.11-raw"},
		{"chef", "aix", "7.3", "raw", "chef-aix-7.3-raw"},
		{"chef", "freebsd", "12", "raw", "chef-freebsd-12-raw"},
		// Generic products (platform_version == "pv" normalized to "generic")
		{"chef", "linux", "pv", "raw", "chef-linux-generic-raw"},
		{"chef", "linux-kernel2", "pv", "raw", "chef-linux-kernel2-generic-raw"},
		// Custom prefix
		{"myorg", "el", "9", "yum", "myorg-el-9-yum"},
		// chef-ice style: arch is package format, platformVersion is CPU arch
		// These still use the package-format branch which keeps platformVersion in name
		{"chef-ice", "linux", "x86_64", "yum", "chef-ice-linux-x86_64-yum"},
		{"chef-ice", "linux", "x86_64", "apt", "chef-ice-linux-x86_64-apt"},
		{"chef-ice", "linux", "x86_64", "raw", "chef-ice-linux-x86_64-raw"},
		{"chef-ice", "windows", "x86_64", "raw", "chef-ice-windows-x86_64-raw"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RepoName(tt.prefix, tt.platform, tt.platformVersion, tt.repoType)
			if got != tt.want {
				t.Errorf("RepoName(%q, %q, %q, %q) = %q, want %q",
					tt.prefix, tt.platform, tt.platformVersion, tt.repoType, got, tt.want)
			}
		})
	}
}
