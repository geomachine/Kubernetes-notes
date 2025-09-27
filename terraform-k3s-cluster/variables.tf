variable "vm_count" {
  description = "Number of VMs to create"
  type        = number
  default     = 2
}

variable "vm_name_prefix" {
  description = "Prefix for VM names"
  type        = string
  default     = "k3s-node"
}

variable "vm_memory" {
  description = "Memory per VM in MB"
  type        = number
  default     = 2048
}

variable "vm_vcpu" {
  description = "vCPU per VM"
  type        = number
  default     = 2
}

variable "vm_image" {
  description = "Path to CentOS 7 cloud image"
  type        = string
  default     = "/var/lib/libvirt/images/CentOS-7-x86_64.qcow2"
}
