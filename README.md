# chef-pkg

`chef-pkg` is a CLI for querying the Progress Chef commercial downloads API, downloading packages, and uploading them to artifact repositories like Nexus and Artifactory.

## Install

```sh
make build
make install
```

`make build` writes the binary to `bin/chef-pkg`. `make install` installs it to `GOBIN` or `GOPATH/bin`.

## Configuration

`chef-pkg` reads configuration from, in order:

1. CLI flags
2. Environment variables with the `CHEFPKG_` prefix
3. `~/.chef-pkg.toml`

Common settings:

- `chef.license_id`
- `chef.base_url`
- `chef.channel`
- `download.dest`
- `download.concurrency`
- `nexus.url`, `nexus.username`, `nexus.password`, `nexus.gpg_keypair`, `nexus.gpg_passphrase`
- `artifactory.url`, `artifactory.token`, `artifactory.username`, `artifactory.password`

Show the resolved config:

```sh
chef-pkg configure --show
```

## Commands

### List

```sh
chef-pkg list products
chef-pkg list versions --product chef
chef-pkg list packages --product chef --version 18
chef-pkg list packages --product chef --version latest --platform ubuntu
```

### Download

```sh
chef-pkg download packages --product chef --version 18
chef-pkg download packages --product chef --version latest --platform ubuntu
chef-pkg download packages --product chef --version all --arch aarch64
```

The default channel is `stable`.

### Upload

Upload commands can fetch from the Chef API and create repositories when needed.

```sh
chef-pkg upload nexus --product chef --version 18 --fetch --create-repos
chef-pkg upload artifactory --product chef --version 18 --fetch --create-repos
```

Repository names are prefixed with `chef` by default so platform and architecture stay grouped under one Chef-owned repo naming scheme.

### Clean

```sh
chef-pkg clean packages --product chef --version 18
```

There is also a hidden Nexus cleanup command for removing Chef-owned Nexus repositories.

### Raw API explorer

Use the raw API explorer to inspect Chef downloads endpoints directly:

```sh
chef-pkg raw get /current/chef/versions/all
chef-pkg raw get /current/chef/packages --query v=18.9.4
chef-pkg raw get /current/chef-ice/packages --query p=linux --query m=linux
```

The `--query` flag is combined with the required `license_id` automatically.

## Notes

- Secrets are redacted in `chef-pkg configure --show`.
- Major-only versions like `18` resolve to the full `18.x.y` release line.
- The default release channel is `stable`.

