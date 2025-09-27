# Kubernetes Cluster Setup with Kustomize

This repository demonstrates deploying multiple applications in Kubernetes using **Kustomize**, a built-in `kubectl` tool for managing multi-resource configurations.

Included apps:

1. **hello-app** – a simple HTTP echo service
2. **headlamp** – a web-based Kubernetes dashboard

---

## **Folder Structure**

```
k8s/
├─ kustomization.yaml      # Root kustomize file
├─ hello-app/
│  ├─ deployment.yaml
│  └─ service.yaml
└─ headlamp/
   ├─ deployment.yaml
   └─ service.yaml
```

* Each app has its **own folder** containing Deployment and Service manifests.
* The root `kustomization.yaml` references all resources.

---

## **Step 1: Root Kustomization**

`k8s/kustomization.yaml`:

```yaml
resources:
  - hello-app/deployment.yaml
  - hello-app/service.yaml
  - headlamp/deployment.yaml
  - headlamp/service.yaml
```

---

## **Step 2: Apply All Resources**

From the `k8s/` folder:

```bash
kubectl apply -k .
```

* This will create **all Deployments and Services** at once.
* Any updates to the manifests can be reapplied with the same command.

---

## **Step 3: Check Status**

Check the Deployments and Pods:

```bash
kubectl get deployments
kubectl get pods
```

Check the Services:

```bash
kubectl get svc
```

---

## **Step 4: Access Applications**

* **hello-app** (NodePort service)

  ```bash
  curl http://<NodeIP>:<hello-service-nodeport>
  ```

  You should see:

  ```
  Hello from Kubernetes!
  ```

* **headlamp** (NodePort service on port 30081)

  ```bash
  http://<NodeIP>:30081
  ```

> Replace `<NodeIP>` with `localhost` if using Minikube or Docker Desktop.

---

## **Step 5: Updating Applications**

Any changes to deployments (e.g., new image, replica count) can be applied with:

```bash
kubectl apply -k .
```

Kubernetes will automatically rollout updates.

---

## **Optional: Environment Overlays**

You can create overlays for different environments (`dev`, `staging`, `prod`) using patches.
Example:

```
k8s/overlays/dev/kustomization.yaml
```

```yaml
resources:
  - ../../base/hello-app
  - ../../base/headlamp

patchesStrategicMerge:
  - hello-app-replicas.yaml
```

`hello-app-replicas.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-app
spec:
  replicas: 1
```

Apply dev environment:

```bash
kubectl apply -k overlays/dev
```

---

## ✅ Benefits of this Kustomize Setup

* Single command (`kubectl apply -k .`) for deploying multiple apps.
* Clean separation of apps and environments.
* Easy to scale, patch, or update without editing base manifests.
* Fully industry-standard practice for multi-app Kubernetes clusters.

---

## **Step 6: Destroy / Cleanup Resources**

To remove all resources deployed via Kustomize:

```bash
kubectl delete -k .
```

* This deletes all Deployments, Services, and other resources listed in your Kustomize file.

Optional: Check that everything is removed:

```bash
kubectl get pods
kubectl get deployments
kubectl get svc
```

If you want to delete **everything in the current namespace** (use with caution):

```bash
kubectl delete all --all
```

---