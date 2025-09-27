# Download the CentOS image
data "http" "centos_image" {
  url = "https://cloud.centos.org/centos/7/images/CentOS-7-x86_64-GenericCloud.qcow2"
}

resource "local_file" "centos_image" {
  content  = data.http.centos_image.response_body
  filename = "/var/lib/libvirt/images/CentOS-7-x86_64.qcow2"
}

# Master VM
resource "libvirt_volume" "master_disk" {
  name   = "k3s-master.qcow2"
  source = "/var/lib/libvirt/images/CentOS-7-x86_64.qcow2"
  format = "qcow2"
  depends_on = [local_file.centos_image]
}

resource "libvirt_cloudinit_disk" "master_ci" {
  name      = "master-cloudinit.iso"
  pool      = "default"
  user_data = <<-EOF
    #cloud-config
    runcmd:
      - curl -sfL https://get.k3s.io | sh -
  EOF
}

resource "libvirt_domain" "master" {
  name   = "k3s-master"
  memory = 2048
  vcpu   = 2

  network_interface {
    network_name   = "default"
    wait_for_lease = true
  }

  disk {
    volume_id = libvirt_volume.master_disk.id
  }

  disk {
    volume_id = libvirt_cloudinit_disk.master_ci.id
  }
}

# Agent VM
resource "libvirt_volume" "agent_disk" {
  name   = "k3s-agent.qcow2"
  source = "/var/lib/libvirt/images/CentOS-7-x86_64.qcow2"
  format = "qcow2"
  depends_on = [local_file.centos_image]
}

resource "libvirt_cloudinit_disk" "agent_ci" {
  name      = "agent-cloudinit.iso"
  pool      = "default"
  user_data = <<-EOF
    #cloud-config
    runcmd:
      - sleep 30
      - curl -sfL https://get.k3s.io | K3S_URL=https://${libvirt_domain.master.network_interface[0].addresses[0]}:6443 K3S_TOKEN=$(ssh centos@${libvirt_domain.master.network_interface[0].addresses[0]} sudo cat /var/lib/rancher/k3s/server/node-token) sh -
  EOF
}

resource "libvirt_domain" "agent" {
  name   = "k3s-agent"
  memory = 2048
  vcpu   = 2

  network_interface {
    network_name   = "default"
    wait_for_lease = true
  }

  disk {
    volume_id = libvirt_volume.agent_disk.id
  }

  disk {
    volume_id = libvirt_cloudinit_disk.agent_ci.id
  }

  depends_on = [libvirt_domain.master]
}

