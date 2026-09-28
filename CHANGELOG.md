# Changelog

## Unreleased

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
