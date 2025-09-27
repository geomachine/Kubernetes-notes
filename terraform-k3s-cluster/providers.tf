terraform {
  required_providers {
    libvirt = {
      source  = "dmacvicar/libvirt"
      version = "~> 0.7"
    }
    http = {
      source  = "hashicorp/http"
      version = "~> 3.2"
    }
  }
}

provider "libvirt" {
  uri = "qemu:///system"
}

provider "http" {}