locals {
  master_host  = var.master_host
  worker_hosts = var.worker_hosts
  ssh_users    = var.ssh_users
  ssh_key      = abspath(var.ssh_private_key_path)
}

# 0) Ensure SSH key exists
resource "null_resource" "ensure_ssh_key" {
  provisioner "local-exec" {
    command = "test -f ${local.ssh_key} || (echo 'SSH private key not found at ${local.ssh_key}'; exit 1)"
  }
}

# 1) Initialize master node locally (Terraform runs on master)
resource "null_resource" "master_init_local" {
  depends_on = [null_resource.ensure_ssh_key]

  provisioner "local-exec" {
    command = <<EOT
sudo swapoff -a
sudo sed -i.bak '/ swap / s/^/#/' /etc/fstab
sudo modprobe br_netfilter
sudo bash -c 'cat > /etc/sysctl.d/k8s.conf <<EOF
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF'
sudo sysctl --system

if [ ! -f /etc/kubernetes/admin.conf ]; then
  sudo kubeadm init --pod-network-cidr=${var.pod_network_cidr} --ignore-preflight-errors=all
fi

mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config

kubectl taint nodes --all node-role.kubernetes.io/master- || true
kubectl apply -f ${var.cni_manifest_url} || true

# Save join command locally
kubeadm token create --print-join-command > ./join_cmd.sh
chmod +x ./join_cmd.sh
EOT
  }
}

# 2) Join worker nodes
resource "null_resource" "join_workers" {
  for_each = { for idx, ip in local.worker_hosts : ip => idx }
  depends_on = [null_resource.master_init_local]

  # Upload join script
  provisioner "file" {
    source      = "./join_cmd.sh"
    destination = "/tmp/join_cmd.sh"

    connection {
      type        = "ssh"
      host        = each.key
      user        = local.ssh_users[each.key]
      private_key = file(local.ssh_key)
      timeout     = "5m"
    }
  }

  # Run join script remotely
  provisioner "remote-exec" {
    connection {
      type        = "ssh"
      host        = each.key
      user        = local.ssh_users[each.key]
      private_key = file(local.ssh_key)
      timeout     = "10m"
    }

    # inline = [
    #   "sudo swapoff -a || true",
    #   "sudo sed -i.bak '/ swap / s/^/#/' /etc/fstab || true",
    #   "sudo chmod +x /tmp/join_cmd.sh || true",
    #   "sudo /tmp/join_cmd.sh || true"
    # ]
    inline = [
      "swapoff -a || true",
      "chmod +x /tmp/join_cmd.sh || true",
      "/tmp/join_cmd.sh || true"
    ]
  }
}

# 3) Show nodes
resource "null_resource" "show_nodes" {
  depends_on = [null_resource.join_workers]

  provisioner "local-exec" {
    command = "kubectl get nodes -o wide"
  }
}
