# Changelog

## 0.9.0

Bakes FortressEdge v0.9.0, a security release. A shared ISO no longer
carries the Vault secret ID: FortressEdge 0.9 refuses `vault_secret_id`
in `fortress.yml` unless the ISO is said to be one edge's own. Give each
edge its own secret ID, bound to its address, in one of two ways:

- in its cloud-init `user-data`, with `fortressedge_iso` shared by every
  edge:

  ```yaml
  #cloud-config
  fqdn: edge1.example.com
  vault_secret_id: 2f1b...
  ```

- or in an ISO baked for that edge alone, with the new
  `vault_secret_in_iso`:

  ```terraform
  vault_secret_id     = var.edge1_secret_id # sensitive
  vault_secret_in_iso = true
  ```

  That ISO is a credential, written `0600`: keep it as you keep the
  keys, and never publish it.

A configuration that sets `vault_secret_id` without `vault_secret_in_iso`
fails at plan time. The other attributes are unchanged: FortressEdge 0.7
and 0.8 changed only the policy, which this provider does not bake.

## 0.6.0

Bakes FortressEdge v0.6.0, whose edges can share their certificates
through Vault. New attributes, all optional:

```terraform
vault           = "https://vault.internal.example:8200"
vault_ca        = file("platform-root.pem")
vault_mount     = "edge-certs"
vault_path      = "prod/public"
vault_role_id   = var.edge_role_id
vault_secret_id = var.edge_secret_id # sensitive
```

## 0.5.0

Bakes FortressEdge v0.5.0, which gets dark nodes back within a fraction
of a second of a restart and answers 503 while they are away.
`fortress.yml` is unchanged: no attribute changes.

## 0.4.0

Bakes FortressEdge v0.4.0, whose clock keeps in step with its NTP
servers while it runs and follows them only when a majority agree.
`ntp` is a list of servers, in place of one:

```terraform
ntp = ["ntp11.metas.ch", "ntp12.metas.ch", "ntp13.metas.ch"]
```

## 0.3.0

Bakes FortressEdge v0.3.0. `access_log`, `access_log_max_size`, and
`access_log_max_files` are gone: FortressEdge 0.3 moved them from
`fortress.yml` to its policy, so `fortressctl apply` turns the access
log on and off on a running edge, for every site or one. Drop them from
`fortressedge_iso` and put them in `policy.yml`:

```yaml
access_log: true
access_log_max_size: 8MiB
access_log_max_files: 3
```

The Proxmox example boots the ISO from `scsi0` (virtio-scsi), which the
BIOS reads about 2 seconds faster than an IDE CD.

## 0.2.0

`fortressedge_iso` takes the edge's settings as attributes, one per
`fortress.yml` key (`client_ca`, `acme`, `acme_ca`, `ntp`,
`renew_interval`, `quic`, `access_log`, `access_log_max_size`,
`access_log_max_files`), in place of `config`. A setting that fails the
edge's check is an error on its attribute. Bakes FortressEdge v0.2.0.

Replace `config = file("fortress.yml")` with its keys:

```terraform
client_ca = file("client-ca.pem")
acme      = "https://vault.example.com:8200/v1/pki/acme/directory"
```

## 0.1.0

First release. `fortressedge_iso` bakes a FortressEdge release ISO, from
a URL pinned by its SHA-256 or a local file, with `fortress.yml`, checked
at plan time as the edge checks it. It returns the ISO's path, SHA-256,
and a file name that changes with its content. Downloads and baked ISOs
are cached, in the user's cache directory or `cache_dir`.
