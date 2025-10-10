# Kubernetes Essentials

This repository contains essential commands and operations for managing Kubernetes clusters using the `kubectl` command-line tool.

## Commands

### Managing Pods

- `kubectl get pods`: Print out information about all of the running pods.
- `kubectl exec -it [pod_name] [cmd]`: Execute the given command in a running pod.
- `kubectl logs [pod_name]`: Print out logs from the given pod.
- `kubectl delete pod [pod_name]`: Deletes the given pod.
- `kubectl apply -f [config file name]`: Tells Kubernetes to process the configuration file.
- `kubectl describe pod [pod_name]`: Print out some information about the running pod.
- `kubectl delete pods --all`: delete all pods in default namespace
- `kubectl delete pods --all -n <namespace>`: delete all pods in custom namespace


### Managing Deployments

- `kubectl get deployments`: List all the running deployments.
- `kubectl describe deployment [depl name]`: Print out details about a specific deployment.
- `kubectl apply -f [config file name]`: Create a deployment out of a configuration file.
- `kubectl delete deployment [depl_name]`: Delete a deployment.
- `kubectl rollout restart deployment [depl_name]`: Get a deployment to restart all pods. Will use the latest version of an image if the pod spec has a tag of 'latest'.
- ` kubectl apply -f .` : Make deployment for all the deployment files in the directory
- `kubectl delete deployments --all` : delete all deployments in default namespace
- `kubectl delete deployments --all -n <namespace>` : delete all deployments in custom namespace


### Managing Services

- `kubectl apply -f [depl name]` : Apply the configuration using the following command:
- `kubectl get services` : Verify the service deployment:
- `kubectl describe service [service name]` : Obtain detailed information about the service:
- `kubectl logs service [service name]` : Check the logs for the service:
- `kubectl delete services --all` : delete all services default namespace
- `kubectl delete services --all -n <namespace>` : delete all services custom namespace


### After Updating K8S specs of deployment's image version to the latest

- `kubectl rollout restart deployment [depl_name]`: Updating the Image Used By a Deployment - Best Engineering Method.
  
## Usage

You can use these commands to effectively manage and monitor your Kubernetes clusters. Make sure to replace `[pod_name]` and `[depl_name]` with the actual names of your pods and deployments.

## License

This project is licensed under the [MIT License](https://opensource.org/licenses/MIT).
