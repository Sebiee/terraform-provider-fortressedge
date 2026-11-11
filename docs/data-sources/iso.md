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
  release_url    = "https://github.com/Sebiee/fortressedge/releases/download/v0.9.0/fortressedge-v0.9.0.iso"
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
- `ntp` (List of String) The time servers the edge keeps its clock to, each `host` or `host:port`, at most eight: three or more, so one that is wrong is outvoted. Default: `pool.ntp.org`.
- `quic` (Boolean) Dark nodes may also connect over QUIC on UDP 443. Default: `false`.
- `release_path` (String) A release ISO on this machine, instead of `release_url`.
- `release_sha256` (String) The release ISO's SHA-256, from the release's `SHA256SUMS`. A download that does not match is refused. Required with `release_url`.
- `release_url` (String) Where to download the release ISO, such as its GitHub release asset. Downloads are cached by checksum.
- `renew_interval` (String) How often ACME certificates are checked and renewed once due, a duration such as `4h`. Default: `4h`.
- `vault` (String) The https URL of a Vault whose KV v2 path the edges that serve the same names share their certificates, ACME account, and challenges in. Needs `vault_mount`, `vault_path`, and `vault_role_id`, and each edge's own secret ID: in its cloud-init `user-data`, or in an ISO baked for it alone (`vault_secret_id` with `vault_secret_in_iso`). Default: none, each edge keeps its own.
- `vault_ca` (String) The PEM CA, or chain, that signed Vault's HTTPS certificate. Default: the system roots.
- `vault_mount` (String) The KV v2 mount, such as `edge-certs`.
- `vault_path` (String) These edges' path in the mount, such as `prod/public`: one path for the edges that serve the same names.
- `vault_role_id` (String) The AppRole the edge logs in to Vault with. Its policy should allow `vault_path` and nothing else.
- `vault_secret_id` (String, Sensitive) This edge's own AppRole secret ID, for an ISO baked for this one edge alone: valid only with `vault_secret_in_iso`. The ISO is then a credential, written `0600`: keep it as you keep the keys, never publish it, and bind the secret ID to the edge's address (`cidr_list`) so a copy is useless elsewhere. Without it, each edge's secret ID goes in its cloud-init `user-data`.
- `vault_secret_in_iso` (Boolean) This ISO is one edge's own, so it may carry `vault_secret_id`. A shared ISO holds no secret. Default: `false`.

### Read-Only

- `file_name` (String) A name for the upload that changes with its content: `fortressedge-<12 hex digits>.iso`.
- `id` (String) The baked ISO's SHA-256.
- `path` (String) The baked ISO, in the cache directory.
- `sha256` (String) The baked ISO's SHA-256.
