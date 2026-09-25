# Terraform Provider for FortressEdge

Bakes [FortressEdge](https://github.com/Sebiee/fortressedge) ISOs on the
machine that runs Terraform. `fortressedge_iso` takes a release ISO,
pinned by its checksum, and `fortress.yml`, and returns the baked ISO's
path and SHA-256 for the platform's upload, such as
`proxmox_virtual_environment_file`. The bake is fortressedge's own
(`github.com/Sebiee/fortressedge/bake`), so it is the same bytes as
`fortressctl bake`, on any OS, and the checksum is known at plan time.

```terraform
data "fortressedge_iso" "prod" {
  release_url    = "https://github.com/Sebiee/fortressedge/releases/download/v0.1.0/fortressedge-v0.1.0.iso"
  release_sha256 = "…"
  config         = file("${path.module}/fortress.yml")
}
```

Documentation: [docs/](docs/index.md), and [examples/proxmox](examples/proxmox/main.tf).

## Versions

The provider validates and bakes `fortress.yml` with the fortressedge
module version in `go.mod`, so bake an ISO with the provider release that
matches its edge release: fortressedge v0.2.x with provider v0.2.x.

## Development

```sh
make test       # unit tests
make testacc    # through Terraform itself (needs terraform on PATH)
make build      # ./terraform-provider-fortressedge
```

## Releasing

A `v*` tag runs goreleaser (`.goreleaser.yml`), which builds every
platform, signs `SHA256SUMS` with GPG, and publishes a GitHub release the
Terraform Registry picks up. Once, before the first release:

1. Make a GPG key for signing (RSA or DSA; the registry does not take
   ed25519): `gpg --full-generate-key`, then
   `gpg --armor --export-secret-keys <id>` and `gpg --armor --export <id>`.
2. In this repository's Settings, Secrets and variables, Actions, add
   `GPG_PRIVATE_KEY` (the armored private key) and `PASSPHRASE`.
3. In the Terraform Registry, Publish, Provider, sign in with GitHub, add
   the public key under the namespace's signing keys, and pick this
   repository.
4. Make `go.mod` require a published fortressedge instead of the local
   checkout: `go mod edit -dropreplace github.com/Sebiee/fortressedge`
   and `go get github.com/Sebiee/fortressedge@<version>`. The release
   workflow refuses to build while the local replace is there.
5. `git tag v0.1.0 && git push origin v0.1.0`.
