---
page_title: "fortressedge_iso Data Source - fortressedge"
subcategory: ""
description: |-
  A FortressEdge release ISO with fortress.yml baked in, written from these attributes, as fortressctl bake makes it.
---

# fortressedge_iso (Data Source)

A FortressEdge release ISO with `fortress.yml` baked in, written from
these attributes, as `fortressctl bake` makes it. Each attribute is the
`fortress.yml` key of the same name; one left out keeps the edge's
default. The same release and settings give the same bytes, so `sha256`
is known at plan time. The settings are checked at plan time, as the edge
checks them.

The policy (`block`, `exempt`, `limits`, the access log, tracing, and
per-site settings) is not baked: `fortressctl apply` puts it on a
running edge.

## Example Usage

With [bpg/proxmox](https://registry.terraform.io/providers/bpg/proxmox):

```terraform
data "fortressedge_iso" "prod" {
  release_url    = "https://github.com/Sebiee/fortressedge/releases/download/v0.3.0/fortressedge-v0.3.0.iso"
  release_sha256 = "…" # from the release's SHA256SUMS
  client_ca      = file("${path.module}/client-ca.pem")
  acme           = "https://vault.example.com:8200/v1/pki/acme/directory"
  acme_ca        = file("${path.module}/vault-ca.pem")
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

- `client_ca` (String) The PEM CA certificate that signs the dark-node, operator, and log-reader certificates: exactly one certificate.

### Optional

- `acme` (String) The ACME directory URL. Default: Let's Encrypt.
- `acme_ca` (String) The PEM CA that signed the ACME directory's HTTPS certificate. Default: the system roots.
- `ntp` (String) The time source for the boot clock sync, `host` or `host:port`. Default: `pool.ntp.org`.
- `quic` (Boolean) Dark nodes may also connect over QUIC on UDP 443. Default: `false`.
- `release_path` (String) A release ISO on this machine, instead of `release_url`.
- `release_sha256` (String) The release ISO's SHA-256, from the release's `SHA256SUMS`. A download that does not match is refused. Required with `release_url`.
- `release_url` (String) Where to download the release ISO, such as its GitHub release asset. Downloads are cached by checksum.
- `renew_interval` (String) How often ACME certificates are checked and renewed once due, a duration such as `4h`. Default: `4h`.

### Read-Only

- `file_name` (String) A name for the upload that changes with its content: `fortressedge-<12 hex digits>.iso`.
- `id` (String) The baked ISO's SHA-256.
- `path` (String) The baked ISO, in the cache directory.
- `sha256` (String) The baked ISO's SHA-256.
