---
page_title: "Provider: FortressEdge"
description: |-
  Bakes FortressEdge ISOs on the machine that runs Terraform.
---

# FortressEdge Provider

[FortressEdge](https://github.com/Sebiee/fortressedge) is a locked-down
edge that boots from an ISO. Whom an edge trusts (its client CA, its ACME
server) is `fortress.yml`, baked into the release ISO. This provider
bakes it where Terraform runs, as `fortressctl bake` does, byte for byte:
the same release and `fortress.yml` give the same ISO on any machine, so
its checksum is known at plan time and an unchanged configuration uploads
nothing.

Each machine then boots that ISO with a cloud-init (NoCloud) drive giving
its `fqdn` and address, which platforms such as Proxmox write from their
own settings. See [provisioning](https://github.com/Sebiee/fortressedge/blob/main/docs/provisioning.md).

## Example Usage

```terraform
terraform {
  required_providers {
    fortressedge = {
      source = "sebiee/fortressedge"
    }
  }
}

provider "fortressedge" {}

data "fortressedge_iso" "prod" {
  release_url    = "https://github.com/Sebiee/fortressedge/releases/download/v0.1.0/fortressedge-v0.1.0.iso"
  release_sha256 = "…" # from the release's SHA256SUMS
  config         = file("${path.module}/fortress.yml")
}
```

## Schema

### Optional

- `cache_dir` (String) Where downloaded releases and baked ISOs are kept. Default: `fortressedge` in the user's cache directory (`$XDG_CACHE_HOME` or `~/.cache` on Linux, `~/Library/Caches` on macOS, `%LocalAppData%` on Windows).
