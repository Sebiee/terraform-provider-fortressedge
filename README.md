# Terraform Provider for FortressEdge

The [FortressEdge](https://github.com/Sebiee/fortressedge) provider bakes
edge ISOs on the machine that runs Terraform. Its data source,
`fortressedge_iso`, takes a FortressEdge release ISO, pinned by its
checksum, and a `fortress.yml`, and returns the baked ISO's path and
SHA-256 for your platform to upload.

A bake is reproducible: the same release and `fortress.yml` give the same
bytes on any machine, identical to `fortressctl bake`. The checksum is
therefore known at plan time, and an unchanged configuration uploads
nothing. `fortress.yml` is checked at plan time, as the edge checks it.

## Usage

```terraform
terraform {
  required_providers {
    fortressedge = { source = "sebiee/fortressedge" }
  }
}

data "fortressedge_iso" "edge" {
  release_url    = "https://github.com/Sebiee/fortressedge/releases/download/v0.1.0/fortressedge-v0.1.0.iso"
  release_sha256 = "…" # from the release's SHA256SUMS
  config         = file("${path.module}/fortress.yml")
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
its own release. Use the provider version that matches the FortressEdge
release you bake.

## Development

Requires Go, and Terraform for the acceptance tests.

```sh
make build      # ./terraform-provider-fortressedge
make test       # unit tests
make testacc    # acceptance tests, through Terraform
```

## Releasing

Pushing a `v*` tag runs [goreleaser](https://goreleaser.com), which
builds every platform, signs `SHA256SUMS` with GPG, and publishes the
GitHub release the Terraform Registry reads. The workflow needs two
repository secrets: `GPG_PRIVATE_KEY`, an ASCII-armored RSA key whose
public half is registered with the Terraform Registry, and `PASSPHRASE`.

```sh
git tag v0.1.0
git push origin v0.1.0
```

## License

MIT
