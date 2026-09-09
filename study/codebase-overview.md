# Kubernetes Codebase — High-Level Overview

This is the real `kubernetes/kubernetes` source repo. Plain-English map of what's where.

## The "brains" — where the actual logic lives

- **`cmd/`** — The entry point for every binary Kubernetes ships. Each subfolder is one program you can run: `kube-apiserver`, `kube-scheduler`, `kubelet`, `kube-proxy`, `kubectl`, `kubeadm`, etc. Think of this as "the list of apps in this project." These files are tiny — they just wire things up and call into `pkg/`.

- **`pkg/`** — The actual implementation. This is the biggest chunk of real logic. Examples:
  - `pkg/scheduler` — decides which node a pod runs on
  - `pkg/kubelet` — runs on every node, manages containers
  - `pkg/proxy` — kube-proxy's networking logic
  - `pkg/registry` — how the API server stores objects
  - `pkg/controller` — the control loops that keep the cluster in the desired state

- **`plugin/`** — Pluggable pieces of the API server: admission controllers (things that check/modify requests before they're saved, like `imagepolicy`, `podnodeselector`, `noderestriction`) and auth plugins.

## Shared building blocks

- **`staging/src/k8s.io/`** — Reusable libraries: `client-go`, `apimachinery`, `apiserver`, `kubelet`, `cli-runtime`, etc. "Staging" means: these are developed *inside* this repo, but each one is also published as its own separate public repo (e.g. `client-go` is what other projects import to talk to a Kubernetes cluster). Symlinked into `vendor/` so the rest of the code can use them normally.

- **`vendor/`** — Third-party dependencies (other people's Go libraries) needed to build, copied in so builds are self-contained.

- **`api/`** — Formal API definitions/specs (OpenAPI spec, rules about how the API is allowed to change).

## Testing

- **`test/`** — All the tests that don't live next to their code: end-to-end tests (`e2e`), integration tests, conformance tests (checks any "Kubernetes-compliant" cluster must pass), node-level tests, kubeadm tests, etc.

## Running / deploying it

- **`cluster/`** — Shell scripts to actually stand up a cluster (mostly on GCE) and add-ons — more "ops scripts" than app code.
- **`build/`** — Docker/build tooling to compile everything and package it into container images/release artifacts.

## Dev tooling / meta

- **`hack/`** — A huge folder of shell scripts developers use daily: run tests, generate code, check style, update dependencies, verify things. Not shipped to users — it's for people working *on* Kubernetes.
- **`CHANGELOG/`** — Release notes, one file per version.
- **`docs/`, `logo/`, `LICENSES/`, `third_party/`** — Documentation, branding, and license bookkeeping.
- **`.github/`** — GitHub Actions/CI config, issue templates.
- Root files like `go.mod`, `go.sum`, `go.work` — Go's dependency manifest (what version of what library this project needs).

## The simplest mental model

1. **`cmd/`** = "what programs exist" (thin wrappers)
2. **`pkg/`** = "what those programs actually do" (the real logic)
3. **`staging/`** = "shared toolkits" used both internally and by outside projects
4. **`plugin/`** = "optional add-on checks" for the API server
5. **`test/`** = "does it actually work"
6. **`cluster/` + `build/`** = "how to deploy/build it"
7. **`hack/`** = "scripts for people developing Kubernetes itself"
8. **`vendor/`** = "other people's code we depend on"
