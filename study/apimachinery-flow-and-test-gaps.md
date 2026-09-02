# API Machinery: Request Flow + Untested-Package Scopes

## Part 1 — The complete flow of a request through API Machinery

A single `kubectl apply -f pod.yaml` (or any client request) moves through these stages.
Each stage is a real package in this checkout — paths given relative to their repo root.
Every stage below includes a concrete "where you'd actually see this in a running cluster" example.

```
Client (kubectl / controller / client-go)
   │  HTTP request, JSON/YAML/protobuf body
   ▼
[1] Routing & discovery         apiserver/pkg/endpoints/discovery/
   │  matches URL to a registered GroupVersionResource
   ▼
[2] Authentication              apiserver/pkg/authentication/
   │  who is this? (cert, token, etc.) → user.Info
   ▼
[3] Authorization                apiserver/pkg/authorization/
   │  is this user allowed to do this verb on this resource?
   ▼
[4] Decoding                    apimachinery/pkg/runtime/  (scheme.go, codec.go, serializer/)
   │  raw bytes → versioned Go struct, via the negotiated serializer
   ▼
[5] Conversion (external→internal)  apimachinery/pkg/conversion/, generated zz_generated.conversion.go
   │  e.g. v1.Pod → api.Pod (the version-agnostic internal representation)
   ▼
[6] Defaulting                  pkg/apis/*/v1/defaults.go, zz_generated.defaults.go
   │  fills in omitted fields
   ▼
[7] Admission — mutating         apiserver/pkg/admission/  + plugin/pkg/admission/*
   │  plugins can rewrite the object (defaulting-like side effects, webhooks, policies)
   ▼
[8] Validation                   pkg/apis/*/validation/
   │  structural correctness checks; rejects the request if invalid
   ▼
[9] Admission — validating        apiserver/pkg/admission/  (ValidatingWebhook, ResourceQuota, etc.)
   │  final accept/reject gate, no further mutation allowed
   ▼
[10] Generic REST storage         apiserver/pkg/registry/generic/, apiserver/pkg/registry/rest/
   │    request-agnostic Create/Update/Delete/Watch semantics every resource reuses
   │    (endpoints/handlers/{create,get,update,delete,patch,watch}.go call into this)
   ▼
[11] Storage interface             apiserver/pkg/storage/  (interfaces.go: Versioner, Interface)
   │    abstracts "some key/value store", independent of etcd specifics
   ▼
[12] etcd3 backend + watch cache   apiserver/pkg/storage/etcd3/, apiserver/pkg/storage/cacher/
   │    actual persistence; cacher serves List/Watch from an in-memory replica of etcd
   ▼
[13] Encoding (internal→external)  apimachinery/pkg/runtime/ + serializer/ (reverse of steps 4-5)
   │    response object converted back to the version the client asked for, then serialized
   ▼
Client receives response
```

### Stage-by-stage: where you'd actually see this happen

1. **Routing & discovery** — Run `kubectl get pods`. Before it ever sends the GET, kubectl asks the apiserver "what does `pods` map to?" (group `""`, version `v1`, resource `pods`) and caches the answer in `~/.kube/cache/discovery/`. Delete that cache directory and the next `kubectl` command is visibly slower — that's discovery re-running.
2. **Authentication** — Every node's kubelet authenticates to the apiserver using its own client certificate (`/var/lib/kubelet/pki/kubelet-client-current.pem`); a human running `kubectl` authenticates via whatever's in `~/.kube/config` (a token, a cert, or an `exec:` plugin like `aws eks get-token` / `gke-gcloud-auth-plugin`).
3. **Authorization** — `kubectl auth can-i create pods --namespace dev` literally invokes this stage directly (a `SubjectAccessReview`) without performing the actual action — it's the RBAC check your `Role`/`RoleBinding` YAML defines.
4. **Decoding** — When you `kubectl apply -f pod.yaml`, kubectl sends the manifest as JSON over HTTP; the apiserver's negotiated serializer (this is exactly `runtime/serializer/recognizer`, one of the untested packages below) figures out the body is JSON (vs. protobuf, which is what `client-go`-based controllers use internally for efficiency) and decodes it.
5. **Conversion** — You write `apiVersion: apps/v1`; internally the apiserver converts your `Deployment` into its unversioned internal struct before doing anything else with it. You can see the versioned side of this any time you fetch the same object as two different versions, e.g. `kubectl get deployment nginx -o yaml` vs. hitting an older API version if the resource still supports one.
6. **Defaulting** — Write a Pod spec with no `restartPolicy` and no `terminationGracePeriodSeconds`. `kubectl get pod -o yaml` afterward shows `restartPolicy: Always` and `terminationGracePeriodSeconds: 30` — you never wrote those, defaulting did.
7. **Admission — mutating** — Istio's sidecar injector is the textbook example: it's a `MutatingWebhookConfiguration` that intercepts every Pod create in a labeled namespace and injects an `envoy` sidecar container into `pod.spec.containers` before it's ever stored.
8. **Validation** — Try to create a Pod with `resources.requests.cpu: "-1"`. You get an immediate 422/`Invalid` error — that's the structural validation stage rejecting it before it reaches admission's validating phase or storage.
9. **Admission — validating** — Set a `ResourceQuota` of `pods: "5"` in a namespace, then try to create a 6th pod: it's rejected here, not at validation (the pod spec itself is perfectly valid) and not at storage (it never gets that far).
10. **Generic REST storage** — This is why `kubectl create` on almost any resource type — `Pod`, `ConfigMap`, `CustomResourceDefinition` you've never heard of — behaves consistently (409 Conflict on duplicate name, 404 on missing, same `--dry-run` behavior). One shared `Store` implementation backs all of them; each resource just plugs in its own naming/validation "strategy."
11. **Storage interface** — This abstraction is *why* `kube-apiserver --etcd-servers=...` is the only place etcd is mentioned — nothing above this layer knows or cares that the backing store is etcd rather than something else.
12. **etcd3 + watch cache** — Run `ETCDCTL_API=3 etcdctl get /registry/pods/default/my-pod` on a control-plane node and you'll see the literal stored protobuf-ish object. Meanwhile, every one of your 500 nodes' kubelets watching for pods scheduled to them is served by the **cacher** (an in-memory replica), not 500 separate etcd watches — this is the single biggest reason large clusters don't fall over from watch load.
13. **Encoding** — `kubectl get pod nginx -o yaml` vs. `-o json` — same stored object, same internal representation, encoded differently on the way out based on what you asked for.

### Where CRDs and aggregation fit in
- **CRDs**: e.g. cert-manager's `Certificate` CRD, or Istio's `VirtualService` — these are handled by `apiextensions-apiserver` (separate repo) which dynamically builds a REST handler using this *same* pipeline (steps 10-13), representing the object as `runtime.Unstructured` (`apimachinery/pkg/apis/meta/v1/unstructured/`) instead of a compiled-in Go struct, since the apiserver has no Go type for `Certificate` at compile time.
- **Aggregation**: `metrics-server` is the most common real example — `kubectl top nodes` works because `APIService` registers `v1beta1.metrics.k8s.io` with the main apiserver, which authenticates/authorizes the request (steps 1-3) then proxies it straight to the metrics-server pod, skipping steps 4-13 locally entirely.

---

## Part 2 — Packages with zero unit tests

Method: scanned `apimachinery/pkg/` and `apiserver/pkg/` for directories that contain
`.go` source files but **no** `*_test.go` file in that same directory. Filtered out
noise (fuzzer packages, `testing`/`apitesting` helper packages, `install` one-liners,
pure generated API-version type packages) to leave genuine, logic-bearing candidates.

### apimachinery — candidates

| Package | Files | What it does | Real-life example in a cluster |
|---|---|---|---|
| `pkg/api/equality/semantic.go` | 1 (52 lines) | Defines `Semantic` — the `conversion.Equalities` used project-wide for deep-equal comparisons of API objects. | The **Deployment controller** uses this to decide "did the pod template actually change?" before triggering a new rollout — this is why editing a Deployment's `metadata.annotations` alone (unrelated to pod spec) doesn't trigger a rolling restart. |
| `pkg/api/safe/safe.go` | 1 (59 lines) | Nil-safe accessor helpers for common API fields. | Anywhere controller code reads something like `pod.Status.StartTime` without first checking `pod.Status != nil` — this is what stops that from panicking. |
| `pkg/runtime/serializer/recognizer/recognizer.go` | 1 (128 lines) | Auto-detects which serializer (JSON/YAML/protobuf/CBOR) a byte stream is in. | The literal reason `kubectl apply -f pod.yaml` (YAML) and an internal controller's protobuf request both decode correctly against the same apiserver endpoint — see step 4 above. |
| `pkg/types/` (`namespacedname.go`, `nodename.go`, `patch.go`, `uid.go`) | 4 (146 lines) | Core scalar types: `UID`, `NamespacedName`, `NodeName`, `PatchType`. | `UID` is what the **garbage collector** matches against `ownerReferences` to know a `ReplicaSet`'s pods should be deleted when the `ReplicaSet` is; `NamespacedName` is the literal work-queue key type every controller (`Deployment`, `Job`, etc.) uses ("default/my-app"); `NodeName` is the field the **scheduler** sets (`pod.spec.nodeName = "worker-3"`) to bind a pod to a node. |
| `pkg/util/httpstream/` + `spdy/` + `wsstream/` | 6 files | Low-level streaming protocol implementation. | This is the actual wire protocol under `kubectl exec -it mypod -- bash` — your keystrokes and the shell's stdout travel over this. |
| `pkg/util/portforward/` | 1 file | Client-side `kubectl port-forward` implementation. | `kubectl port-forward pod/my-app 8080:80` — this package is what's running while that command is open. |
| `pkg/util/remotecommand/` | 1 file | Client-side `kubectl exec`/`attach` implementation. | Same command as the httpstream example above — this is the layer above it that frames stdin/stdout/stderr/resize. |
| `pkg/apis/meta/v1beta1/validation/` | 1 file | Validation logic for the (mostly legacy) v1beta1 meta types. | Still linked into any code path validating deprecated `v1beta1` meta options — a quiet legacy corner most clusters no longer exercise directly, which is exactly why it's easy for a regression here to go unnoticed. |

### apiserver — candidates

| Package | Files | What it does | Real-life example in a cluster |
|---|---|---|---|
| `pkg/registry/generic/matcher.go`, `options.go`, `storage_decorator.go` | 3 (171 lines) | The generic label/field-selector matcher and storage decorator every resource's List/Watch goes through. | Every time you run `kubectl get pods -l app=nginx` or `--field-selector status.phase=Running`, **this is the code that filters the results** — it's shared by every resource type, not reimplemented per-type. `storage_decorator.go` is also why `Event` objects (high write volume, short TTL) can be configured with different storage behavior than everything else. |
| `pkg/storage/value/encrypt/identity/identity.go` | 1 (57 lines) | The no-op "identity" transformer in the encryption-at-rest chain. | This is the **default path for every `Secret` written to etcd** on any cluster that hasn't configured an `EncryptionConfiguration` — i.e. most clusters. It's the fallback that makes encryption-at-rest optional rather than mandatory. |
| `pkg/storage/errors/storage.go` | 1 (128 lines) | Storage-layer error types (`NewKeyNotFoundError`, `NewConflictErr`, etc.) and `Is*` helpers. | The "the object has been modified; please apply your changes to the latest version" error you get from `kubectl edit` on a stale object is `NewConflictErr` surfacing all the way up to your terminal; deleting an already-deleted pod twice quickly surfaces `NewKeyNotFoundError` as a 404. |
| `pkg/endpoints/warning/`, `pkg/warning/` | 1 file each | Recorder for HTTP `Warning` response headers. | The `Warning: 299 - "v1beta1 ... is deprecated"` message `kubectl` sometimes prints when you use a deprecated API version comes from here. |
| `pkg/util/dryrun/` | 1 file | Context helpers for propagating dry-run mode through the request. | `kubectl apply --dry-run=server -f pod.yaml` — this runs the *entire* pipeline above (admission, validation, etc.) but this package is what stops step 10-12 from actually persisting anything. |
| `pkg/authentication/user/` | 2 files | `user.Info` interface + `DefaultInfo` struct. | Check any audit log entry or admission webhook request — the `userInfo.username` field (e.g. `system:serviceaccount:default:my-sa`, or your own identity from `kubectl auth whoami`) is populated from this type. |

### Recommended entry point
Start with **`apimachinery/pkg/types/`** or **`pkg/api/safe/`** — smallest, purest, zero existing tests, no risk of breaking behavior since you're only *adding* test files. Then graduate to **`apiserver/pkg/registry/generic/matcher.go`** once comfortable — it's still self-contained but sits in genuinely important, in-charter-scope machinery (generic CRUD, the thing behind every `kubectl get -l ...`), which makes it a more compelling contribution.

Note: this scan checked for the *presence* of a test file per directory, not coverage percentage — a directory with one thin test file wouldn't show up here even if most functions in it are untested. Treat this as a first pass, not a coverage report.
