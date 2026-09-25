---
page_title: "fortressedge_iso Data Source - fortressedge"
subcategory: ""
description: |-
  A FortressEdge release ISO with fortress.yml baked in, as fortressctl bake makes it.
---

# fortressedge_iso (Data Source)

A FortressEdge release ISO with `fortress.yml` baked in, as `fortressctl
bake` makes it. The same release and `fortress.yml` give the same bytes,
so `sha256` is known at plan time. `fortress.yml` is checked at plan time
as the edge checks it: a missing `client_ca`, an unknown key, or a policy
key (`block`, `exempt`, `limits`, which `fortressctl apply` puts on a
running edge) is an error.

## Example Usage

With [bpg/proxmox](https://registry.terraform.io/providers/bpg/proxmox):

```terraform
data "fortressedge_iso" "prod" {
  release_url    = "https://github.com/Sebiee/fortressedge/releases/download/v0.1.0/fortressedge-v0.1.0.iso"
  release_sha256 = "…" # from the release's SHA256SUMS
  config         = file("${path.module}/fortress.yml")
}

resource "proxmox_virtual_environment_file" "edge_iso" {
  node_name    = "pve"
  datastore_id = "local"
  content_type = "iso"
  source_file {
    path      = data.fortressedge_iso.prod.path
    file_name = data.fortressedge_iso.prod.file_name
    checksum  = data.fortressedge_iso.prod.sha256
  }
}
```

A build of your own: `release_path = "../fortressedge/out/fortressedge.iso"`
in place of `release_url` and `release_sha256`.

## Schema

### Required

- `config` (String) The `fortress.yml` to bake: `acme`, `acme_ca`, `client_ca`, `ntp`, and the rest.

### Optional

- `release_path` (String) A release ISO on this machine, instead of `release_url`.
- `release_sha256` (String) The release ISO's SHA-256, from the release's `SHA256SUMS`. A download that does not match is refused. Required with `release_url`.
- `release_url` (String) Where to download the release ISO, such as its GitHub release asset. Downloads are cached by checksum.

### Read-Only

- `file_name` (String) A name for the upload that changes with its content: `fortressedge-<12 hex digits>.iso`.
- `id` (String) The baked ISO's SHA-256.
- `path` (String) The baked ISO, in the cache directory.
- `sha256` (String) The baked ISO's SHA-256.
