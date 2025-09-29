#!/usr/bin/env bash
set -euo pipefail

echo ">>> Stopping Kubernetes, containerd, and Docker services..."
sudo systemctl stop kubelet || true
sudo systemctl stop docker || true
sudo systemctl stop containerd || true
sudo systemctl stop cri-docker || true

echo ">>> Disabling services..."
sudo systemctl disable kubelet || true
sudo systemctl disable docker || true
sudo systemctl disable containerd || true
sudo systemctl disable cri-docker || true

echo ">>> Removing Kubernetes packages..."
sudo apt-get purge -y kubeadm kubectl kubelet kubernetes-cni kube* || true

echo ">>> Removing Docker and container runtimes..."
sudo apt-get purge -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin docker-compose || true

echo ">>> Removing snap Kubernetes (if installed)..."
sudo snap remove --purge k8s || true
sudo snap remove --purge kubectl || true
sudo snap remove --purge microk8s || true

echo ">>> Cleaning leftover config and state..."
sudo rm -rf /etc/kubernetes \
            /var/lib/etcd \
            /var/lib/kubelet \
            /var/lib/dockershim \
            /var/lib/docker \
            /var/lib/containerd \
            /var/run/docker.sock \
            ~/.kube

echo ">>> Autoremoving unused packages..."
sudo apt-get autoremove -y
sudo apt-get autoclean -y

echo ">>> Checking for leftovers..."
dpkg -l | grep -iE 'docker|containerd|kube' || echo "No related packages found."

echo ">>> Wipe complete. Reboot recommended!"
