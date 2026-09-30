# Changelog

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
