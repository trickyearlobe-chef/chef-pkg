# Platform & Architecture Normalization

## Platform Name Normalization

| Chef API | Normalized |
|---|---|
| `el` | `el` |
| `amazon` | `amzn` |
| `sles` | `sles` |
| `opensuse` | `opensuse` |
| `ubuntu` | `ubuntu` |
| `debian` | `debian` |
| `rocky` | `rocky` |
| `alma` | `alma` |
| `fedora` | `fedora` |
| `windows` | `windows` |
| `mac_os_x` | `macos` |

## Platform Version Normalization (apt codenames)

| Distro | Version | Codename |
|---|---|---|
| Ubuntu | `20.04` | `focal` |
| Ubuntu | `22.04` | `jammy` |
| Ubuntu | `24.04` | `noble` |
| Debian | `10` | `buster` |
| Debian | `11` | `bullseye` |
| Debian | `12` | `bookworm` |
| All others | `*` | used as-is |

Unknown versions produce a warning and fall back to the raw version string.

## Architecture Normalization

| Chef API | yum | apt | raw/nuget |
|---|---|---|---|
| `x86_64` | `x86_64` | `amd64` | `x86_64` |
| `aarch64` | `aarch64` | `arm64` | `aarch64` |
| `ppc64le` | `ppc64le` | `ppc64el` | `ppc64le` |
| `ppc64` | `ppc64` | `ppc64` | `ppc64` |
| `i386` | `i386` | `i386` | `i386` |
| `s390x` | `s390x` | `s390x` | `s390x` |

## Repo Type Mapping

| Platform | File extension | Repo type |
|---|---|---|
| `el`, `amazon`, `sles`, `rocky`, `alma`, `fedora`, `opensuse` | `.rpm` | `yum` |
| `ubuntu`, `debian` | `.deb` | `apt` |
| `windows` | `.msi` | `raw` |
| `windows` | `.nupkg` | `nuget` |
| everything else | `*` | `raw` |

## Repo Naming Convention

Pattern: `{prefix}-{normalizedPlatform}{normalizedVersion}-{normalizedArch}-{repoType}`

The repo type suffix is always included.

Examples:

```
chef-el9-x86_64-yum
chef-el8-aarch64-yum
chef-amzn2023-x86_64-yum
chef-sles15-x86_64-yum
chef-ubuntu-jammy-amd64-apt
chef-ubuntu-noble-amd64-apt
chef-debian-bookworm-amd64-apt
chef-windows2019-x86_64-raw
chef-windows2019-x86_64-nuget
chef-macos13-x86_64-raw
```

## Artifact Path Within Repos

Multiple products and versions coexist in the same repo:

```
chef-ice/19.1.158/chef-ice-19.1.158-1.el9.x86_64.rpm
chef/18.4.2/chef-18.4.2-1.el9.x86_64.rpm
inspec/6.8.1/inspec-6.8.1-1.el9.x86_64.rpm
```

## Functions

- `NormalizePlatform(chefPlatform) → string`
- `NormalizePlatformVersion(platform, version) → string`
- `NormalizeArch(repoType, chefArch) → string`
- `RepoType(platform, fileExtension) → string`
- `RepoName(prefix, platform, platformVersion, arch, repoType) → string`
