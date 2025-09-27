# Optional outputs
output "master_ip" {
  value = libvirt_domain.master.network_interface[0].addresses[0]
}

output "agent_ip" {
  value = libvirt_domain.agent.network_interface[0].addresses[0]
}
