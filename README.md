# Terraform Provider for FortressEdge

The [FortressEdge](https://github.com/Sebiee/fortressedge) provider bakes
edge ISOs on the machine that runs Terraform. Its data source,
`fortressedge_iso`, takes a FortressEdge release ISO, pinned by its
checksum, and the edge's settings (its client CA, its ACME server, and
the rest of `fortress.yml`, one attribute each), and returns the baked
ISO's path and SHA-256 for your platform to upload.

A bake is reproducible: the same release and settings give the same bytes
on any machine, identical to `fortressctl bake`. The checksum is
therefore known at plan time, and an unchanged configuration uploads
nothing. The settings are checked at plan time, as the edge checks them.

## Usage

```terraform
terraform {
  required_providers {
    fortressedge = { source = "sebiee/fortressedge" }
  }
}

data "fortressedge_iso" "edge" {
  release_url    = "https://github.com/Sebiee/fortressedge/releases/download/v0.6.0/fortressedge-v0.6.0.iso"
  release_sha256 = "…" # from the release's SHA256SUMS
  client_ca      = file("${path.module}/client-ca.pem")
  acme           = "https://vault.example.com:8200/v1/pki/acme/directory"
  acme_ca        = file("${path.module}/vault-ca.pem")
}

output "iso" {
  value = {
    path   = data.fortressedge_iso.edge.path
    sha256 = data.fortressedge_iso.edge.sha256
  }
}
```

`release_path` takes a local ISO in place of `release_url` and
`release_sha256`. Downloads and baked ISOs are cached in the user's cache
directory, or in the provider's `cache_dir`.

- [Provider documentation](docs/index.md)
- [`fortressedge_iso`](docs/data-sources/iso.md)
- [An edge on Proxmox](examples/proxmox/main.tf), with
  [bpg/proxmox](https://registry.terraform.io/providers/bpg/proxmox)

## Versions

The provider checks and bakes `fortress.yml` with the FortressEdge code of
the release it requires, so its major.minor follows FortressEdge's: provider
v0.5.x reads `fortress.yml` as FortressEdge v0.5.x does. FortressEdge
changes `fortress.yml` only in minor releases, so a provider of the same
minor bakes every patch release of it. Use the provider version whose
major.minor matches the FortressEdge release you bake. It has an attribute
for each key of that release.

## Development

Requires Go, and Terraform for the acceptance tests.

```sh
make build      # ./terraform-provider-fortressedge
make test       # unit tests
make testacc    # acceptance tests, through Terraform
```

## Releasing

Dependabot opens a pull request of its own for each FortressEdge release.
Merging it releases the provider: the release workflow tags the next
version, FortressEdge's major.minor with the provider's next patch, and
publishes it. A release for a change of the provider's own is a tag
pushed by hand. If a FortressEdge release moves its frp or yamux pins, CI
says so, and `scripts/check-pins.sh --fix` copies them.

A release runs [goreleaser](https://goreleaser.com), which
builds every platform, signs `SHA256SUMS` with GPG, and publishes the
GitHub release the Terraform Registry reads. Each zip also carries a build
provenance attestation, signed by GitHub
(`gh attestation verify <zip> --repo Sebiee/terraform-provider-fortressedge`).
The workflow needs two repository secrets: `GPG_PRIVATE_KEY`, an
ASCII-armored RSA key whose public half is registered with the Terraform
Registry, and `PASSPHRASE`.

A fix of the provider's own, after v0.1.1:

```sh
git tag -a v0.1.2 -m v0.1.2
git push origin v0.1.2
```

## License

MIT
