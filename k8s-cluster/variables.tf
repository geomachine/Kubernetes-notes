variable "master_host" {
  description = "IP of the master node"
  type        = string
}

variable "worker_hosts" {
  description = "List of worker node IPs"
  type        = list(string)
}

variable "ssh_users" {
  description = "Map of IP addresses to SSH usernames"
  type        = map(string)
}

variable "ssh_private_key_path" {
  description = "Path to the SSH private key used to connect to all nodes"
  type        = string
  default     = "/home/kraken/.ssh/k8s_cluster_key"
}

variable "pod_network_cidr" {
  description = "CIDR range for Kubernetes pods (used by CNI)"
  type        = string
  default     = "10.244.0.0/16"
}

variable "cni_manifest_url" {
  description = "URL of the CNI manifest to apply on the cluster"
  type        = string
  default     = "https://raw.githubusercontent.com/flannel-io/flannel/master/Documentation/kube-flannel.yml"
}
