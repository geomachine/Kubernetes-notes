# 🚀 Deploy Headlamp in Kubernetes

This manifest deploys [Headlamp](https://github.com/headlamp-k8s/headlamp), a web-based Kubernetes dashboard, inside your cluster.

## 📄 What the YAML does

* **Deployment**

  * Creates a Deployment named `headlamp`.
  * Runs **1 replica** of the Headlamp container.
  * Uses the image: `ghcr.io/headlamp-k8s/headlamp:latest`.
  * Exposes container port `4466`.

* **Service**

  * Creates a Service named `headlamp`.
  * Type is `NodePort` so it’s accessible outside the cluster.
  * Maps:

    * Service port: **4466**
    * Container port: **4466**
    * NodePort: **30081** (so you can access via `<NodeIP>:30081`)

---

## ⚡ Deployment Steps

1. **Save the manifest** as `headlamp.yml`.

2. **Apply it to your cluster:**

   ```bash
   kubectl apply -f headlamp.yml
   ```

3. **Check the Deployment:**

   ```bash
   kubectl get deployments
   kubectl get pods -l app=headlamp
   ```

4. **Check the Service:**

   ```bash
   kubectl get svc headlamp
   ```

5. **Access Headlamp in your browser:**

   ```
   http://<NodeIP>:30081
   ```

   * If using Docker Desktop or Minikube, `NodeIP` is usually `localhost`.
   * If running on a remote cluster, use the node’s external IP.

---