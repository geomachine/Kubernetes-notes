### **Step 1: Create a Deployment YAML**

Create a file called `deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: hello-app
  template:
    metadata:
      labels:
        app: hello-app
    spec:
      containers:
      - name: hello-app
        image: hashicorp/http-echo
        args:
        - "-text=Hello from Kubernetes!"
        ports:
        - containerPort: 5678
```

**What this does:**

* Creates a **Deployment** named `hello-app`.
* Runs **2 replicas** (2 pods) of the container.
* Uses the `hashicorp/http-echo` container to just return a simple text response.
* Exposes **port 5678** inside the pod.

---

### **Step 2: Create a Service YAML**

Create a file called `service.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: hello-service
spec:
  selector:
    app: hello-app
  type: NodePort
  ports:
    - protocol: TCP
      port: 80
      targetPort: 5678
```

**What this does:**

* Creates a **Service** to expose the pods to the outside.
* Maps **port 80 on the Service** to **port 5678 in the pods**.
* Type `NodePort` makes it accessible on a port of your node.

---

### **Step 3: Apply the YAML files**

```bash
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
```

---

### **Step 4: Check that it’s running**

```bash
kubectl get pods
kubectl get deployments
kubectl get services
```

* You should see **2 pods** running.
* The service will show a `NodePort` like `30001` (could be different).

---

### **Step 5: Access your app**

* If using Docker Desktop or Minikube:

```bash
curl http://localhost:<NodePort>
```

* Example:

```bash
curl http://localhost:30001
# Output: Hello from Kubernetes!
```

Or open in your browser: `http://localhost:30001`

---

✅ That’s it! You now have a **simple web app running in Kubernetes** with a Deployment and a Service.

---