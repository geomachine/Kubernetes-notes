# 📝 K3s + Tailscale Setup Guide

## **Overview**

This guide explains how to:

1. Configure Tailscale in **kernel networking mode**.
2. Configure K3s to bind to your **Tailscale IP**.
3. Ensure Flannel uses Tailscale (`tailscale0`) for pod networking.
4. Access the cluster from **Headlamp** or other clients.

---

## **Step 1: Ensure Tailscale kernel mode networking**

By default, Tailscale may run in **userspace mode**, which doesn’t create `tailscale0` for the kernel. K3s requires a kernel interface for `--flannel-iface`.

1. Stop Tailscale (optional but safe):

```bash
sudo systemctl stop tailscaled
```

2. Override the systemd service to force kernel mode:

```bash
sudo systemctl edit tailscaled
```

Add:

```ini
[Service]
ExecStart=
ExecStart=/usr/bin/tailscaled --state=/var/lib/tailscale/tailscaled.state --socket=/var/run/tailscale/tailscaled.sock
```

3. Reload systemd and restart Tailscale:

```bash
sudo systemctl daemon-reload
sudo systemctl restart tailscaled
```

4. Verify the interface exists:

```bash
ip addr show tailscale0
```

You should see something like:

```
19: tailscale0: <POINTOPOINT,MULTICAST,NOARP,UP,LOWER_UP> ...
    inet 100.64.0.4/32 scope global tailscale0
```

---

## **Step 2: Configure K3s to use Tailscale IP**

### 2a. Stop K3s

```bash
sudo systemctl stop k3s
```

### 2b. Edit K3s systemd service

```bash
sudo systemctl edit k3s
```

Add:

```ini
[Service]
ExecStart=
ExecStart=/usr/local/bin/k3s server \
  --node-ip=100.64.0.4 \
  --advertise-address=100.64.0.4 \
  --tls-san=100.64.0.4 \
  --flannel-iface=tailscale0
```

* `--node-ip` = your Tailscale IP.
* `--advertise-address` = IP other nodes should connect to.
* `--tls-san` = include in server TLS cert.
* `--flannel-iface` = tells Flannel to route pods via Tailscale.

### 2c. Reload systemd

```bash
sudo systemctl daemon-reload
```

---

## **Step 3: Clean up old K3s state**

> Important if K3s failed previously or has local state tied to another IP.

```bash
sudo rm -rf /var/lib/rancher/k3s/server
sudo rm -rf /etc/rancher/k3s
```

---

## **Step 4: Start K3s**

```bash
sudo systemctl restart k3s
sudo systemctl status k3s
```

Verify it is active. If it fails, check:

```bash
sudo journalctl -u k3s -f
```

---

## **Step 5: Verify K3s cluster**

```bash
sudo k3s kubectl get nodes -o wide
sudo k3s kubectl -n kube-system get pods -o wide
```

Make sure:

* Node shows `100.64.0.4`.
* Flannel pods are using `tailscale0`.
* Cluster is healthy.

---

## **Step 6: Use Headlamp**

1. Copy your **custom kubeconfig** (not the default one in `/etc/rancher/k3s/k3s.yaml`):

```bash
sudo cp /etc/rancher/k3s/k3s.yaml ~/k3s-tailscale.yaml
sudo nano ~/k3s-tailscale.yaml
```

2. Change the server IP to your Tailscale IP:

```yaml
clusters:
- cluster:
    server: https://100.64.0.4:6443
```

3. Open Headlamp (`http://127.0.0.1:9090`) → **Add Cluster → Import kubeconfig** → Paste this file.

Now you can see your nodes, pods, and Flannel network.

---

## **Step 7: Optional sanity checks**

```bash
ip addr show tailscale0
sudo k3s kubectl get pods -o wide -A
ping 100.64.0.4   # test Tailscale reachability
```

---

### ✅ Notes / Gotchas

* **Usermode Tailscale** → K3s cannot see an interface → cluster fails to start.
* **Custom K3s kubeconfig** is mandatory when binding to Tailscale IP.
* **Flannel** must know the interface (`--flannel-iface=tailscale0`).
* Cleaning previous state (`/var/lib/rancher/k3s/server`) avoids IP conflicts.

---

This document is basically your **“next time setup playbook”**.