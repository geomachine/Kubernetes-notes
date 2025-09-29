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
   #YOU WILL HIT ERROR HERE. THEN USE THIS PROCEEDURE AND FORWARD:
   ```
The error you're encountering during `sudo kubeadm init --pod-network-cidr=192.168.0.0/16` indicates that the preflight check failed because the file `/proc/sys/net/bridge/bridge-nf-call-iptables` does not exist. This is related to the Linux kernel's bridge netfilter module, which is required for Kubernetes networking to function properly with certain Container Network Interface (CNI) plugins like Calico. The module (`br_netfilter`) is either not loaded or not available on your system. Here's how to resolve this issue and proceed with the Kubernetes installation.

---

### Step-by-Step Resolution

#### Step 1: Load the `br_netfilter` Module
The `br_netfilter` kernel module is necessary for Kubernetes to handle bridged network traffic correctly.

1. **Check if the module is available**:
   Run the following command to see if `br_netfilter` is loaded:
   ```bash
   lsmod | grep br_netfilter
   ```
   If no output is returned, the module is not loaded.

2. **Load the module**:
   Load the `br_netfilter` module manually:
   ```bash
   sudo modprobe br_netfilter
   ```

3. **Verify the module is loaded**:
   Run the `lsmod` command again:
   ```bash
   lsmod | grep br_netfilter
   ```
   You should see output like:
   ```
   br_netfilter           32768  0
   bridge                176128  1 br_netfilter
   ```

4. **Make the module load on boot**:
   To ensure `br_netfilter` loads automatically on system startup, add it to the kernel modules configuration:
   ```bash
   echo "br_netfilter" | sudo tee -a /etc/modules-load.d/kubernetes.conf
   ```

#### Step 2: Enable Bridge Netfilter Settings
Kubernetes requires specific sysctl settings for bridge networking.

1. **Set the required sysctl parameters**:
   Create or edit the sysctl configuration file for Kubernetes:
   ```bash
   sudo nano /etc/sysctl.d/99-kubernetes.conf
   ```
   Add the following lines:
   ```
   net.bridge.bridge-nf-call-iptables = 1
   net.bridge.bridge-nf-call-ip6tables = 1
   net.ipv4.ip_forward = 1
   ```
   Save and exit (`Ctrl+O`, `Enter`, `Ctrl+X` in `nano`).

2. **Apply the sysctl settings**:
   Reload the sysctl configuration:
   ```bash
   sudo sysctl --system
   ```

3. **Verify the settings**:
   Check that the required parameters are set:
   ```bash
   sysctl net.bridge.bridge-nf-call-iptables net.bridge.bridge-nf-call-ip6tables net.ipv4.ip_forward
   ```
   Expected output:
   ```
   net.bridge.bridge-nf-call-iptables = 1
   net.bridge.bridge-nf-call-ip6tables = 1
   net.ipv4.ip_forward = 1
   ```

#### Step 3: Retry `kubeadm init`
Now that the `br_netfilter` module is loaded and sysctl settings are configured, retry the `kubeadm init` command:
```bash
sudo kubeadm init --pod-network-cidr=192.168.0.0/16
```

This should proceed without the preflight error. The command will take a few minutes to complete.

---

### If the Error Persists
If you still encounter the same error or a related issue, try the following:

1. **Check kernel version**:
   Ensure your kernel supports `br_netfilter`:
   ```bash
   uname -r
   ```
   Ubuntu 20.04 or 22.04 should have a compatible kernel (e.g., 5.x). If you're using an older or custom kernel, you may need to update it:
   ```bash
   sudo apt update && sudo apt install -y linux-generic
   ```

2. **Install bridge utilities**:
   Ensure the `bridge-utils` package is installed, as it provides tools for managing bridge interfaces:
   ```bash
   sudo apt install -y bridge-utils
   ```

3. **Debug with verbose output**:
   Run `kubeadm init` with higher verbosity to get more details:
   ```bash
   sudo kubeadm init --pod-network-cidr=192.168.0.0/16 --v=5
   ```
   Review the output for specific clues about the failure.

4. **Ignore the error (not recommended)**:
   As a last resort, you can bypass the preflight check, but this is risky and may cause networking issues later:
   ```bash
   sudo kubeadm init --pod-network-cidr=192.168.0.0/16 --ignore-preflight-errors=FileContent--proc-sys-net-bridge-bridge-nf-call-iptables
   ```
   Only use this if you're certain the networking setup is correct.

---

### Post-Initialization Steps
Once `kubeadm init` succeeds, follow the output instructions to:
1. Set up the `kubectl` configuration:
   ```bash
   mkdir -p $HOME/.kube
   sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
   sudo chown $(id -u):$(id -g) $HOME/.kube/config
   ```

2. Install a CNI plugin (e.g., Calico):
   ```bash
   kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.25.0/manifests/calico.yaml
   ```

3. Verify the cluster:
   ```bash
   kubectl get nodes
   kubectl get pods -n kube-system
   ```

---

### Additional Notes
- **CNI Compatibility**: The `--pod-network-cidr=192.168.0.0/16` is specific to Calico. If you plan to use another CNI (e.g., Flannel), adjust the CIDR (e.g., `10.244.0.0/16` for Flannel).
- **System Requirements**: Ensure your system meets Kubernetes requirements (2 CPUs, 2GB RAM, swap disabled, container runtime installed).
- **Reset if needed**: If you encounter repeated issues, reset the cluster and start over:
  ```bash
  sudo kubeadm reset
  ```

If you encounter further errors or need help with the next steps, share the error output or let me know what specific issue you're facing!
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

-------------
Final debug and fix:
Nice 🎉 your **control plane is Ready** now, Calico is running, CoreDNS is running, and your cluster is healthy.
But your **app pods (nginx, hello-app, headlamp)** are stuck in `Pending`.

👉 This is because your cluster currently has **only the control plane node**, and by default Kubernetes **taints control-plane/master nodes** so they don’t schedule regular workloads.

---

### 🔧 Fix: Allow workloads on control-plane node

Since you don’t have worker nodes yet, you need to untaint the control plane so it can also run pods:

```bash
kubectl taint nodes --all node-role.kubernetes.io/control-plane-
```

If you’re on Kubernetes v1.24+, sometimes it uses `node-role.kubernetes.io/master` instead:

```bash
kubectl taint nodes --all node-role.kubernetes.io/master-
```

That removes the "NoSchedule" taint from the control-plane node.

---

### ✅ After that

Run:

```bash
kubectl get pods -A -o wide
```

You should see your `nginx`, `hello-app`, `headlamp` moving from `Pending` → `ContainerCreating` → `Running`.

---

### ⚡ Next Steps

* If you plan to add **worker nodes**, leave the taint in place and just join workers with the `kubeadm join ...` command.
* If this is just for **learning / dev setup**, untainting control-plane is perfectly fine.

---

Do you want me to also show you how to expose your **NodePort services** (like `nginx` and `headlamp`) in the browser from your Ubuntu host?
