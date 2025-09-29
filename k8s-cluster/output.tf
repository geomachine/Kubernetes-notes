output "master_host" {
  description = "IP of the Kubernetes master node"
  value       = local.master_host
}

output "worker_hosts" {
  description = "List of Kubernetes worker node IPs"
  value       = local.worker_hosts
}
