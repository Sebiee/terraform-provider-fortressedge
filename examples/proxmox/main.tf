# An edge on Proxmox: the ISO baked here, uploaded, and a VM whose
# cloud-init drive gives its name (edge1.example.com) and address.
terraform {
  required_providers {
    fortressedge = { source = "sebiee/fortressedge" }
    proxmox      = { source = "bpg/proxmox" }
  }
}

variable "release_sha256" {
  description = "fortressedge-v0.1.0.iso's line in the release's SHA256SUMS"
  type        = string
}

data "fortressedge_iso" "prod" {
  release_url    = "https://github.com/Sebiee/fortressedge/releases/download/v0.1.0/fortressedge-v0.1.0.iso"
  release_sha256 = var.release_sha256
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

resource "proxmox_virtual_environment_vm" "edge1" {
  node_name = "pve"
  name      = "edge1"
  cpu { cores = 2 }
  memory { dedicated = 1024 }
  cdrom {
    file_id   = proxmox_virtual_environment_file.edge_iso.id
    interface = "ide0"
  }
  disk {
    datastore_id = "local-lvm"
    interface    = "virtio0"
    size         = 1
  }
  boot_order = ["ide0"]
  network_device { bridge = "vmbr0" }
  initialization {
    datastore_id = "local-lvm"
    interface    = "ide2"
    dns {
      domain  = "example.com"
      servers = ["10.0.0.53"]
    }
    ip_config {
      ipv4 {
        address = "10.0.10.21/24"
        gateway = "10.0.10.1"
      }
    }
  }
}
