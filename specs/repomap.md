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
| `darwin` | `macos` |

## Platform Version Normalization

Applied in order:

1. If version is `"pv"` → `"generic"` (all platforms).
2. For apt platforms, map version to codename:

| Distro | Version | Codename |
|---|---|---|
| Ubuntu | `20.04` | `focal` |
| Ubuntu | `22.04` | `jammy` |
| Ubuntu | `24.04` | `noble` |
| Debian | `10` | `buster` |
| Debian | `11` | `bullseye` |
| Debian | `12` | `bookworm` |

3. All others: used as-is.

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

Pattern: `{prefix}-{normalizedPlatform}-{normalizedVersion}-{repoType}`

Architecture is **not** included in the repo name. Yum and apt repos natively support multiple architectures within a single repo. Raw repos use path-based separation for arch variants (see Artifact Path below).

Examples:

```
chef-el-9-yum
chef-el-8-yum
chef-amzn-2023-yum
chef-sles-15-yum
chef-ubuntu-jammy-apt
chef-ubuntu-noble-apt
chef-debian-bookworm-apt
chef-windows-2019-raw
chef-windows-2019-nuget
chef-macos-13-raw
```

## Artifact Path Within Repos

Multiple products, versions, and architectures coexist in the same repo. Path structure inside the repo:

```
{product}/{version}/{arch}/{filename}
```

Examples:

```
chef-ice/19.1.158/x86_64/chef-ice-19.1.158-1.el9.x86_64.rpm
chef/18.4.2/aarch64/chef-18.4.2-1.el9.aarch64.rpm
inspec/6.8.1/x86_64/inspec-6.8.1-1.el9.x86_64.rpm
```

## Functions

- `NormalizePlatform(chefPlatform) → string`
- `NormalizePlatformVersion(platform, version) → string`
- `NormalizeArch(repoType, chefArch) → string`
- `RepoType(platform, fileExtension) → string`
- `RepoName(prefix, platform, platformVersion, repoType) → string`
