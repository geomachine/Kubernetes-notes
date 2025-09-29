Installing Kubernetes on Ubuntu Linux involves setting up a cluster with all necessary components, including the control plane, worker nodes, and supporting tools like `kubeadm`, `kubectl`, and a container runtime (e.g., containerd or Docker). Below is a step-by-step guide to install a single-node or multi-node Kubernetes cluster on Ubuntu. This assumes a clean Ubuntu 20.04 or 22.04 LTS system, but it can be adapted for other versions. I'll cover both a single-node setup (using `kubeadm`) and prerequisites for a multi-node cluster.

---

### Prerequisites
1. **System Requirements**:
   - **OS**: Ubuntu 20.04 or 22.04 LTS (or later).
   - **Hardware**: Minimum 2 CPUs, 2GB RAM per machine (4GB+ recommended for production-like setups).
   - **Network**: Full network connectivity between all machines in the cluster (for multi-node setups). Ensure ports required by Kubernetes are open (e.g., 6443 for API server, 10250 for Kubelet).
   - **Root Access**: Commands require `sudo` or root privileges.
   - **Swap Disabled**: Kubernetes requires swap to be disabled.

2. **Nodes**:
   - For a single-node cluster, you need one machine (control plane and worker combined).
   - For a multi-node cluster, designate one machine as the control plane and others as worker nodes.

---

### Step-by-Step Installation Guide

#### Step 1: Update and Prepare the System
1. **Update the system**:
   ```bash
   sudo apt update && sudo apt upgrade -y
   ```

2. **Install required packages**:
   Install utilities like `curl` and `apt-transport-https`:
   ```bash
   sudo apt install -y curl apt-transport-https ca-certificates
   ```

3. **Disable swap**:
   Kubernetes requires swap to be disabled for performance reasons.
   ```bash
   sudo swapoff -a
   ```
   To make this permanent, edit `/etc/fstab` and comment out any swap entries:
   ```bash
   sudo nano /etc/fstab
   ```
   Find the swap line (e.g., `/swap.img` or `swapfile`) and comment it out with `#`. Save and exit.

4. **Verify swap is disabled**:
   ```bash
   free -h
   ```
   Ensure the "Swap" row shows 0B used.

---

#### Step 2: Install a Container Runtime
Kubernetes requires a container runtime to manage containers. We'll use **containerd** (recommended by Kubernetes), but you can use Docker if preferred.

1. **Install containerd**:
   ```bash
   sudo apt install -y containerd
   ```

2. **Configure containerd**:
   Create a default configuration file:
   ```bash
   sudo mkdir -p /etc/containerd
   containerd config default | sudo tee /etc/containerd/config.toml
   ```

3. **Enable systemd cgroup driver** (required for Kubernetes):
   Edit the `config.toml` file:
   ```bash
   sudo nano /etc/containerd/config.toml
   ```
   Find the `[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]` section and set:
   ```toml
   SystemdCgroup = true
   ```
   Save and exit.

4. **Restart containerd**:
   ```bash
   sudo systemctl restart containerd
   sudo systemctl enable containerd
   ```

5. **Verify containerd is running**:
   ```bash
   sudo systemctl status containerd
   ```

---

#### Step 3: Install Kubernetes Components
Kubernetes components include `kubeadm`, `kubelet`, and `kubectl`.

1. **Add the Kubernetes apt repository**:
   Add the Kubernetes signing key:
   ```bash
   curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
   ```
   Add the repository:
   ```bash
   echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list
   ```

   **Note**: This installs Kubernetes v1.28. Adjust the version (e.g., `v1.29`) if you need a different one.

2. **Update apt and install Kubernetes components**:
   ```bash
   sudo apt update
   sudo apt install -y kubeadm kubelet kubectl
   ```

3. **Hold package versions** (to prevent unintended upgrades):
   ```bash
   sudo apt-mark hold kubeadm kubelet kubectl
   ```

4. **Verify installations**:
   ```bash
   kubeadm version
   kubectl version --client
   kubelet --version
   ```

---

#### Step 4: Initialize the Kubernetes Cluster (Control Plane)
This step is performed on the **control plane node** only.

1. **Initialize the cluster with kubeadm**:
   Use `kubeadm init` to set up the control plane. Specify a pod network CIDR (we'll use Calico later; adjust if using another CNI like Flannel or Weave).
   ```bash
   sudo kubeadm init --pod-network-cidr=192.168.0.0/16
   ```

   **Note**:
   - This command may take a few minutes.
   - The `--pod-network-cidr` is specific to the CNI plugin (e.g., Calico uses `192.168.0.0/16`, Flannel uses `10.244.0.0/16`).
   - If you encounter errors (e.g., port conflicts), ensure required ports are free (use `netstat -tuln` to check).

2. **Post-initialization steps**:
   After successful initialization, you'll see output with instructions. Follow these to set up `kubectl` for the non-root user:
   ```bash
   mkdir -p $HOME/.kube
   sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
   sudo chown $(id -u):$(id -g) $HOME/.kube/config
   ```

3. **Verify the control plane**:
   Check the cluster status:
   ```bash
   kubectl get nodes
   ```
   The control plane node should appear with a `NotReady` status (until the CNI is installed).

---

#### Step 5: Install a Container Network Interface (CNI)
A CNI plugin is required for pod networking. We'll use **Calico**, but you can choose others like Flannel or Weave.

1. **Install Calico**:
   Apply the Calico manifests:
   ```bash
   kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.25.0/manifests/calico.yaml
   ```

2. **Verify Calico installation**:
   Wait a few moments and check the status of pods in the `kube-system` namespace:
   ```bash
   kubectl get pods -n kube-system
   ```
   Ensure all Calico pods are in the `Running` state.

3. **Check node status**:
   After the CNI is running, the control plane node should transition to `Ready`:
   ```bash
   kubectl get nodes
   ```

---

#### Step 6: Join Worker Nodes (Multi-Node Cluster Only)
If setting up a multi-node cluster, repeat Steps 1–3 on each worker node, then join them to the cluster.

1. **Get the join command**:
   On the control plane node, retrieve the join command (output from `kubeadm init`):
   ```bash
   kubeadm token create --print-join-command
   ```
   This will output something like:
   ```bash
   kubeadm join 192.168.x.x:6443 --token <token> --discovery-token-ca-cert-hash sha256:<hash>
   ```

2. **Run the join command on each worker node**:
   Copy the command and run it with `sudo` on each worker node:
   ```bash
   sudo kubeadm join 192.168.x.x:6443 --token <token> --discovery-token-ca-cert-hash sha256:<hash>
   ```

3. **Verify the cluster**:
   On the control plane, check that all nodes are listed:
   ```bash
   kubectl get nodes
   ```
   All nodes should appear with the `Ready` status.

---

#### Step 7: (Optional) Enable Scheduling on Control Plane (Single-Node Cluster)
For a single-node cluster, the control plane node must also act as a worker. Remove the control plane taint to allow scheduling pods:
```bash
kubectl taint nodes --all node-role.kubernetes.io/control-plane-
```

---

#### Step 8: Test the Cluster
Deploy a sample application to verify the cluster is working:
1. **Create a test deployment**:
   ```bash
   kubectl create deployment nginx --image=nginx
   kubectl scale deployment nginx --replicas=2
   ```

2. **Expose the deployment**:
   ```bash
   kubectl expose deployment nginx --port=80 --type=NodePort
   ```

3. **Check the service**:
   ```bash
   kubectl get svc
   ```
   Note the `NodePort` (e.g., 30000–32767). Access it via `http://<node-ip>:<node-port>`.

4. **Verify pods**:
   ```bash
   kubectl get pods
   ```

---

### Troubleshooting
- **Nodes NotReady**: Check CNI pods (`kubectl get pods -n kube-system`) or container runtime status (`systemctl status containerd`).
- **Kubeadm errors**: Ensure swap is disabled, ports are open, and the correct CIDR is used.
- **Networking issues**: Verify the CNI plugin is installed and running.
- **Logs**: Use `journalctl -u kubelet` or `kubectl logs <pod-name> -n kube-system` for debugging.

---

### Additional Notes
- **Multi-Node Setup**: Ensure all nodes have unique hostnames (`sudo hostnamectl set-hostname <name>`) and update `/etc/hosts` if needed.
- **CNI Alternatives**: For Flannel, use:
  ```bash
  kubectl apply -f https://raw.githubusercontent.com/flannel-io/flannel/master/Documentation/kube-flannel.yml
  ```
  Adjust `--pod-network-cidr` to `10.244.0.0/16` during `kubeadm init`.
- **Upgrades**: To upgrade Kubernetes, use `apt` to install newer versions and follow `kubeadm upgrade` procedures.
- **Cleanup**: To reset the cluster, run `sudo kubeadm reset` on each node and remove `$HOME/.kube/config`.

---

This guide sets up a functional Kubernetes cluster with all core components. For production, consider adding high availability (e.g., multiple control planes), persistent storage, and monitoring. Let me know if you need help with specific configurations or troubleshooting!
