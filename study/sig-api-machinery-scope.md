# SIG API Machinery — Scope of Work

Reference notes for contributing to `kubernetes/kubernetes` under SIG API Machinery.
Source: `community/sig-api-machinery/charter.md`, `community/sig-api-machinery/README.md`.

## In-scope areas and where the code lives

| Area | Definition | Where it lives |
|---|---|---|
| **API server** | The `kube-apiserver` binary — the front door of the control plane; handles all REST requests, auth, and routes them to storage. | `cmd/kube-apiserver/`, `pkg/kubeapiserver/`, `pkg/controlplane/`, `staging/src/k8s.io/apiserver/` |
| **API registration & discovery** | How the apiserver advertises which API groups/versions/resources it serves (`/apis`, `/api` endpoints), and how additional (aggregated) API servers register themselves. | `staging/src/k8s.io/apiserver/pkg/endpoints/discovery/`, `staging/src/k8s.io/kube-aggregator/` |
| **Generic API CRUD semantics** | The shared machinery behind Create/Read/Update/Delete/Watch that every resource type reuses (REST storage interfaces, strategy hooks, etc.) rather than each resource reimplementing it. | `staging/src/k8s.io/apiserver/pkg/registry/`, `pkg/registry/` |
| **Admission control** | Plugins that intercept a request after auth but before persistence, to validate or mutate it (e.g. `imagepolicy`, `podnodeselector`, `noderestriction`). | `plugin/pkg/admission/`, `staging/src/k8s.io/apiserver/pkg/admission/` |
| **Encoding/decoding** | Turning API objects to/from wire formats (JSON, YAML, protobuf, CBOR). | `staging/src/k8s.io/apimachinery/pkg/runtime/serializer/` |
| **Conversion** | Converting objects between API versions (e.g. `v1beta1` ↔ `v1`) and between internal/external representations. | `staging/src/k8s.io/apimachinery/pkg/conversion/`, generated `zz_generated.conversion.go` files |
| **Defaulting** | Filling in default field values for objects that omit them, applied at decode time. | `pkg/apis/*/v1/defaults.go`, generated `zz_generated.defaults.go` |
| **Persistence layer (etcd)** | How objects are actually stored and read from etcd, including the watch cache and storage interface abstraction. | `staging/src/k8s.io/apiserver/pkg/storage/etcd3/`, `staging/src/k8s.io/apiserver/pkg/storage/` |
| **OpenAPI** | Generating and serving the OpenAPI spec describing the entire API surface (used by `kubectl explain`, client generators, validation). | `staging/src/k8s.io/kube-openapi/` (separate repo), `pkg/generated/openapi/` |
| **Informer libraries** | Client-side caching/watch abstraction (`SharedInformer`) that controllers use instead of polling the API server directly. | `staging/src/k8s.io/client-go/informers/`, `staging/src/k8s.io/client-go/tools/cache/` |
| **CustomResourceDefinitions (CRDs)** | Lets users define their own API types without writing a custom apiserver. | separate repo: `kubernetes/apiextensions-apiserver` (mirrored at `staging/src/k8s.io/apiextensions-apiserver/`) |
| **Webhooks** | Admission webhooks (validating/mutating) and conversion webhooks — external services the apiserver calls out to. | `staging/src/k8s.io/apiserver/pkg/admission/plugin/webhook/`, `staging/src/k8s.io/apiextensions-apiserver/.../webhook/` |
| **Garbage collection** | Controller that deletes dependent objects when their owner (via `ownerReferences`) is deleted. | `pkg/controller/garbagecollector/` |
| **Namespace lifecycle** | Controller managing namespace deletion/finalization (ensuring all objects in a namespace are cleaned up before it's removed). | `pkg/controller/namespace/` |
| **Client libraries** | `client-go` — the Go client used by virtually everything (kubectl, controllers, external tools) to talk to the API server. | separate repo: `kubernetes/client-go` (mirrored at `staging/src/k8s.io/client-go/`) |

## Out of scope
- **Contents of individual APIs** (e.g. what fields a `Pod` or `Deployment` has) — owned by **SIG Architecture**, not SIG API Machinery. API Machinery owns the *machinery* that serves and processes any API type, not the type definitions themselves.

## Risk tiers (for picking a first contribution)

- **Low risk / good entry point**: missing `doc.go` files, undocumented functions, unit tests for functions currently only covered by integration/e2e tests, edge-case/invalid-input test coverage.
- **Medium risk**: new self-contained utility libraries, encapsulated behind a flag, no wiring into existing call paths.
- **High risk / needs track record first**: changes to existing encoding/decoding, admission control, conversion, garbage collection, or anything in `cmd/kube-apiserver/app/server.go` — central, high blast-radius, multiple required reviewers.

## Contacts
- Slack: `#sig-api-machinery`
- Mailing list: kubernetes-sig-api-machinery (Google Groups)
- Bug triage team: `@kubernetes/sig-api-machinery-bugs`
- PR reviews team: `@kubernetes/sig-api-machinery-pr-reviews`
- Regular SIG meeting: Wednesdays 11:00 PT, biweekly
- Chairs: David Eads (@deads2k), Federico Bongiovanni (@fedebongio)

## Process reminders
- CLA is signed *after* opening your first PR — the EasyCLA bot comments with a signing link.
- Security vulnerabilities are **never** filed as public issues — report privately per `kubernetes/kubernetes/SECURITY_CONTACTS` / `kubernetes.io/security`.
- Generated files (`zz_generated.*`) are never hand-edited — run `make update`.
- `go.mod`/`go.work` are generated — use `hack/pin-dependency.sh` + `hack/update-vendor.sh`, never `go mod tidy`.
- Every `.go` file needs the license boilerplate header (`hack/boilerplate/boilerplate.go.txt`).
- Disclose AI assistance in the PR description if used.
