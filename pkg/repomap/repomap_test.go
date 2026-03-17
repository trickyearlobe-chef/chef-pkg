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
		{"solaris2", "solaris"},
		{"freebsd", "freebsd"},
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

func TestRepoName(t *testing.T) {
	tests := []struct {
		prefix          string
		platform        string
		platformVersion string
		arch            string
		repoType        string
		want            string
	}{
		{"chef", "el", "9", "x86_64", "yum", "chef-el9-x86_64-yum"},
		{"chef", "el", "8", "aarch64", "yum", "chef-el8-aarch64-yum"},
		{"chef", "amazon", "2023", "x86_64", "yum", "chef-amzn2023-x86_64-yum"},
		{"chef", "amazon", "2", "aarch64", "yum", "chef-amzn2-aarch64-yum"},
		{"chef", "sles", "15", "x86_64", "yum", "chef-sles15-x86_64-yum"},
		{"chef", "ubuntu", "22.04", "x86_64", "apt", "chef-ubuntu-jammy-amd64-apt"},
		{"chef", "ubuntu", "24.04", "aarch64", "apt", "chef-ubuntu-noble-arm64-apt"},
		{"chef", "debian", "12", "x86_64", "apt", "chef-debian-bookworm-amd64-apt"},
		{"chef", "debian", "11", "x86_64", "apt", "chef-debian-bullseye-amd64-apt"},
		{"chef", "windows", "2019", "x86_64", "raw", "chef-windows2019-x86_64-raw"},
		{"chef", "mac_os_x", "12", "x86_64", "raw", "chef-macos12-x86_64-raw"},
		{"chef", "solaris2", "5.11", "sparc", "raw", "chef-solaris5.11-sparc-raw"},
		{"chef", "aix", "7.3", "powerpc", "raw", "chef-aix7.3-powerpc-raw"},
		{"chef", "freebsd", "12", "x86_64", "raw", "chef-freebsd12-x86_64-raw"},
		// Custom prefix
		{"myorg", "el", "9", "x86_64", "yum", "myorg-el9-x86_64-yum"},
	}
	for _, tt := range tests {
		got := RepoName(tt.prefix, tt.platform, tt.platformVersion, tt.arch, tt.repoType)
		if got != tt.want {
			t.Errorf("RepoName(%q, %q, %q, %q, %q) = %q, want %q",
				tt.prefix, tt.platform, tt.platformVersion, tt.arch, tt.repoType, got, tt.want)
		}
	}
}
