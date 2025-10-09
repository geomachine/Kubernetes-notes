# 🧩 K3s Cluster Setup Guide (Single + Multi-node)

This guide walks through setting up a **K3s Kubernetes cluster** — from installing the control plane, connecting worker nodes, setting up **Headlamp** (Kubernetes dashboard), and verifying connectivity using a **whoami** service.

---

## 🖥️ 1. Control Plane (Master Node) Setup

### Step 1: Install K3s (Control Plane)

```bash
curl -sfL https://get.k3s.io | sh -
```

### Step 2: Verify Installation

```bash
kubectl get nodes
```

### Step 3: Install and Access Headlamp (Kubernetes Dashboard)

```bash
curl -s https://raw.githubusercontent.com/headlamp/headlamp/master/install.sh | bash
```

### Step 4: Run Headlamp

```bash
headlamp --kubeconfig ~/.kube/config
```

## Step 5: Access Headlamp UI

```
http://localhost:8080
```

## Step 6: Get Cluster Token and Control Plane IP

```bash
sudo cat /var/lib/rancher/k3s/server/node-token
hostname -I
```


## Step 7: Join Worker Nodes

```bash
sudo k3s-uninstall.sh
K3S_TOKEN=<contol-plane-node-token>
K3S_URL=https://<control-plane-ip>:6443
K3S_NODE_IP=<node-ip>
curl -sfL https://get.k3s.io/ | \
K3S_URL=$K3S_URL \
K3S_TOKEN=$K3S_TOKEN \
K3S_NODE_IP=$K3S_NODE_IP \
sh -
```

## Step 8: Verify Node Join

```
kubectl get nodes
```

## Step 9: Deploy and Test the whoami Service

```bash
kubectl create deployment whoami --image=containous/whoami
kubectl expose deployment whoami --port=80 --type=ClusterIP
```

## Step 10: Test Connectivity with Port Forwarding

```
sudo kubectl port-forward svc/whoami 8080:80
```

## Step 11: Curl the service

```bash
curl http://localhost:8080
```
