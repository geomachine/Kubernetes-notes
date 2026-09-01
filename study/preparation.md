# Interview Prep — Cloud Omnium Limited
### DevOps Role — Interview with Udayan Ghosh (CTO) & Md. Anower Perves (GM, Technology)

*Rebuilt from scratch for this specific interview. The previous version of this file
was prep for an unrelated company (WellDev Services, Mauritius) — wrong stack, wrong
gaps, wrong interviewers. Discard it entirely.*

---

## 1. Who's actually across the table

**Md. Anower Perves is the one who will test you.** Ex-Team Lead DevOps at Vivasoft
and Tirzok, currently GM/Technology at Cloud Omnium. His real background: Big Data
infra (Cassandra, Solr Cloud, OpenStack Swift administration) going back to 2016,
Ansible automation, ScyllaDB/Spark data migration, ELK log analysis, Nagios failover —
and more recently the **Key Deployer of Surokkha** (Bangladesh's COVID vaccine platform)
and a consultant on **VaxEPI**, both real production government systems handling
national-scale concurrency. At Cloud Omnium his stated focus is OpenStack private
clouds (COPCS), Ceph, Kubernetes/OKD/OpenShift, GitOps/Ansible/Terraform, and
Wazuh/ELK/Prometheus/Grafana. **Three days before this interview, he posted publicly
about secrets management being a foundational, not incidental, engineering practice.**
Assume that's not a coincidence — have a real answer ready, not a talking point.

**Udayan Ghosh (CTO)** reads more architecture/leadership than hands-on toolchain —
Java/Golang background, "Software Solutions Architect," blockchain-curious. Expect him
to probe system design judgment, tradeoffs, and how you communicate technical decisions
to non-implementers, more than exact flag syntax.

**Cloud Omnium itself**, per their own posts: sovereign/compliance-driven private cloud
infrastructure, OpenStack-based, resilience-first philosophy ("built for reality, not
the best case"), sustainability as a first-class engineering constraint, secrets
management as foundational. This is **not** an AWS-native shop. Your actual background —
self-hosted K3s, GitOps, encrypted secrets, private mesh networking, resilience under
real incidents — is a much closer match to what they do than to a typical cloud-native
SaaS DevOps posting. Lean into that; don't undersell it by defaulting to AWS-flavored
answers that don't fit their world.

---

## 2. Resume-to-reality check

This is the honest cross-check you asked for. "Verified" means I watched you do this
directly, in detail, this session — not "you probably know it." Everything else is
either outside what I've personally seen (could still be real from other work) or a
genuine gap worth having honest language ready for.

| Resume claim | Status | Notes | What it is, core components & responsibilities |
|---|---|---|---|
| Kubernetes | **Verified, deep** | K3s cluster, multiple apps, StatefulSets, real incident debugging | Container-orchestration platform — schedules, runs, and self-heals containers across many machines.<br>• **kube-apiserver** — front door; every read/write to cluster state goes through it<br>• **etcd** — distributed key-value store; the single source of truth for all cluster state<br>• **scheduler** — decides which node a new pod lands on<br>• **controller-manager** — reconciliation loops that keep actual state matching desired state (Deployments, ReplicaSets, Nodes, etc.)<br>• **kubelet** (per node) — talks to the container runtime to actually start/stop/watch containers<br>• **kube-proxy** (per node) — implements Service networking/load-balancing rules<br>• **container runtime** (containerd) — pulls images, runs containers |
| Docker | **Verified, deep** | K3s cluster, multiple apps, StatefulSets, real incident debugging | Tooling for building, packaging, and running containers.<br>• **dockerd (Engine)** — daemon managing containers/images/networks/volumes<br>• **containerd** — the runtime dockerd delegates to for image pull + container lifecycle<br>• **runc** — low-level OCI runtime that actually creates the container (namespaces/cgroups)<br>• **CLI** — client talking to dockerd's API<br>• **Compose** — declarative multi-container setup on one host |
| Kustomize | **Verified, deep** | Entire GitOps repo structure, overlays | Template-free customization of raw Kubernetes YAML via patches, not a templating language.<br>• **kustomization.yaml** — declares resources, patches, generators for a base or overlay<br>• **base/overlay** — base = shared resources; overlay = environment-specific patches on top<br>• **generators** — ConfigMap/Secret generators (in this repo's case, a KSOPS exec-plugin generator that decrypts SOPS files at build time)<br>• **patches** (strategic-merge / JSON6902) — the actual per-overlay override mechanism |
| Helm | **Verified, deep** | `--enable-helm` chart inflation for Harbor/ArgoCD/Traefik/Grafana | Kubernetes package manager — templated YAML bundled as versioned, parameterized "charts."<br>• **Chart** — templates + `values.yaml` defaults + `Chart.yaml` metadata<br>• **Templates** — Go-templated YAML rendered against values<br>• **Values** — the parameters filling in templates per environment<br>• **Release** — a named, versioned, *in-cluster-tracked* install of a chart — this tracked state is exactly what's **absent** under `helm template` (no install happened), which was the root cause in Story ① |
| ArgoCD / GitOps | **Verified, deep** | App-of-apps pattern, sync policies, custom health checks, real bugs fixed | GitOps continuous-delivery controller — continuously reconciles live cluster state against a Git repo.<br>• **Application (CR)** — one deployable unit: source repo/path, destination cluster/namespace, sync policy<br>• **Application Controller** — the core reconcile loop; diffs live vs. Git, applies sync ops<br>• **Repo Server** — clones the source and renders it (runs `kustomize build`/`helm template` internally)<br>• **API Server** — serves UI/CLI/webhooks<br>• **Redis / Dex** — caching layer / optional SSO bridge<br>• **`automated: {prune, selfHeal}`** — the setting that auto-applies Git changes *and* auto-reverts live/manual drift — the exact mechanism that reverted a live `kubectl set env` in Story ② |
| SOPS + encrypted secrets | **Verified, deep** | KSOPS generators, age encryption, secret-rotation-triggers-rollout pattern | SOPS encrypts individual values inside a structured file while keeping the file's shape readable/diffable in Git; age is the actual key-pair encryption backend it uses.<br>• **SOPS** — generates a random per-file data key, encrypts each leaf value with it, then encrypts that data key once per recipient, plus a MAC for tamper detection<br>• **age keypair** — public key encrypts (safe to commit); private key decrypts (kept off-repo)<br>• **KSOPS** — a Kustomize exec-plugin that shells out to SOPS at build time, so encrypted files can be referenced directly as generators and get decrypted transparently during rendering |
| Least-privilege access design | **Verified, deep** | via Headscale/Tailscale ACLs, not AWS IAM (see gap note below) | The design discipline of granting only the access a given identity actually needs, nothing by default. Enforced here at the network layer (Headscale ACL: identity/tag → allowed destination:port), not via a cloud IAM policy engine — same discipline, different enforcement point. |
| Tailscale / Headscale, WireGuard | **Verified, deep** | Multi-iteration ACL design, live node-list verification, real bugs caught | WireGuard is the actual encrypted point-to-point tunnel protocol; Tailscale automates key exchange/NAT traversal across many machines into one mesh; Headscale is a self-hosted reimplementation of Tailscale's coordination server.<br>• **WireGuard** — kernel-level encrypted tunnel between two peers' public keys<br>• **Headscale (coordinator)** — node registry, issues each node its mesh IP, brokers initial key exchange<br>• **Tailscale client** (per node) — registers via a preauth key, configures the local WireGuard interface<br>• **ACL policy** (on Headscale) — which identities/tags can reach which nodes/ports, evaluated independently of the tunnel itself<br>• **DERP relay** — encrypted fallback path when direct peer-to-peer isn't reachable (e.g. restrictive NAT) |
| Caddy | **Verified, deep** | Edge TLS termination, header-manipulation bugs found and fixed twice | Web server / reverse proxy, notable for automatic HTTPS.<br>• **Caddyfile** — config format, one site block per hostname<br>• **reverse_proxy** — forwards requests upstream; also where header manipulation happens (exactly where both real bugs in your work lived — `X-Forwarded-For`/`Authorization` handling)<br>• **ACME client** (built in) — automatically requests/renews TLS certs per configured hostname<br>• **Middleware directives** (`basic_auth`, `header_up`, etc.) — request/response processing in the proxy chain |
| Prometheus / Grafana | **Verified, deep** | Real dashboard/health-check debugging, false-positive root-cause work | Prometheus collects and stores metrics and evaluates alerts; Grafana visualizes them (and other data sources).<br>• **Prometheus server** — pull-based scraping of `/metrics` endpoints on an interval, stores time series in its own TSDB, evaluates alert/recording rules<br>• **Exporters** (e.g. node-exporter) — translate some system's internal state into Prometheus's metrics format<br>• **Alertmanager** — receives firing alerts, handles dedup/grouping/routing/silencing, sends notifications<br>• **Grafana** — dashboarding/alerting UI, data-source-agnostic |
| GitHub Actions | **Verified, deep** | Full CI/CD pipeline, image tagging, environment secrets | Event-driven CI/CD built into GitHub, workflows defined as YAML in-repo.<br>• **Workflow file** — declares triggers (push/PR/`workflow_dispatch`) and jobs<br>• **Job** — a set of steps run on one runner<br>• **Runner** — the actual compute (hosted or self-hosted) executing steps<br>• **Actions** — reusable packaged steps, official/third-party/custom (e.g. a custom image-tag-bump action)<br>• **Secrets / Environments** — encrypted values scoped to a repo or named environment, gating credential access |
| Bash | **Verified, deep** | Extensive, throughout | Shell scripting — the glue layer composing CLI tools (`kubectl`, `curl`, `docker`, `git`) into repeatable diagnostic/automation sequences. Not a "components" system in the same sense; the equivalent discipline is exit-code checking (`set -e`/`set -o pipefail`), command substitution, and piping tool output into the next tool. |
| Nginx | **Some exposure** | Migrated an existing nginx setup off to Caddy — real but not deep ownership | Web server/reverse proxy, config-driven, no built-in ACME automation (needs Certbot or similar bolted on).<br>• **Master process** — reads config, manages workers, doesn't handle connections itself<br>• **Worker processes** — event-driven, each handles many concurrent connections<br>• **server block** — per-hostname/port config (nginx's equivalent of a Caddy site block)<br>• **location block** — path-based routing within a server block |
| Ansible | **Designed, not executed here** | Real provisioning pattern designed in detail; never watched an actual playbook run | Agentless configuration management — connects over SSH, no persistent agent on managed nodes.<br>• **Inventory** — the list of managed hosts (static or dynamic)<br>• **Playbook** — ordered plays (host group + tasks)<br>• **Tasks / Modules** — each task invokes an idempotent module (same result whether run once or ten times)<br>• **Roles** — reusable bundles of tasks/templates/handlers for one responsibility (e.g. "harden this node")<br>• **Handlers** — tasks that only fire when notified (e.g. restart a service, only if its config actually changed) |
| Loki / Promtail (logging) | **Verified** | Log aggregation stack, referenced in real debugging | Loki is log aggregation built to be queried like Prometheus metrics — label-indexed, not full-text-indexed.<br>• **Loki server** — indexes only metadata labels, stores raw log content cheaply<br>• **Promtail** — the shipping agent; tails log files/container stdout, attaches the right labels, pushes to Loki |
| Golang, Python, TypeScript, SQL | **Not observed** | Never saw application source this session — plausible from other work, just can't back it myself | Languages, not systems — "core components" doesn't map cleanly. If asked, be ready to speak to what each was actually used for in your own work (Go: compiled backend services/CLIs; Python: scripting/automation; TypeScript: typed Node/frontend services; SQL: relational queries) rather than component architecture. |
| GitLab CI, Jenkins | **Not observed** | Entirely GitHub Actions in what I saw — if real, it's from elsewhere | GitLab CI: pipelines as `.gitlab-ci.yml`, executed by **Runners** (shell/docker/kubernetes executors), same stages/jobs model as GitHub Actions. Jenkins: older, self-hosted, plugin-based; **controller** (schedules/serves UI) + **agents** (actual build executors); pipelines as Jenkinsfile (Groovy DSL) or classic UI jobs. |
| Terraform | **Not observed** | Real infra bootstrap I saw was manual shell/cloud-init, not Terraform modules | Declarative IaC via a plan/apply cycle.<br>• **Providers** — plugins that talk to a specific API (AWS, DigitalOcean, Kubernetes, etc.)<br>• **Resources** — the actual infra objects declared in config<br>• **State file** — Terraform's record of what it believes exists, diffed against config on every plan<br>• **plan → apply** — preview the diff, then execute it |
| AWS CDK | **Not observed** | No AWS-native work seen at all | Infra-as-code where you write actual code (TS/Python/etc.) that *generates* CloudFormation templates, rather than writing declarative HCL/YAML directly — CloudFormation is still what actually provisions resources underneath it. |
| Istio | **Not observed** | Traefik was the actual ingress/proxy layer used, not Istio | Service mesh.<br>• **Envoy sidecar** — injected alongside each pod; actually handles all traffic in/out of that pod (mTLS, retries, telemetry)<br>• **Istiod** — control plane; pushes config to every sidecar, handles service discovery and certificate issuance for mTLS |
| OpenTelemetry | **Not observed** | Prometheus/Grafana/Loki was the real stack, not OTel | Vendor-neutral observability instrumentation standard.<br>• **SDKs / instrumentation** — in-app libraries generating traces/metrics/logs<br>• **Collector** — standalone process that receives, batches/processes, and exports telemetry<br>• **Exporters** — plugins shipping data to a specific backend (Prometheus, Jaeger, Datadog, etc.) |
| Datadog | **Not observed** | Same | SaaS observability platform.<br>• **Agent** — host-level collector<br>• **APM tracing libraries** — in-app instrumentation<br>• **Backend** — storage, dashboards, alerting, all hosted<br>• **Integrations** — pre-built collectors for common services |
| HashiCorp Vault | **Not observed** | SOPS+age was the real secrets tool, not Vault | Runtime secrets management — secrets fetched dynamically, not baked into files.<br>• **Storage backend** — where encrypted data actually persists<br>• **Secrets engines** — pluggable (static KV, dynamic DB credentials, PKI, etc.)<br>• **Auth methods** — how a client authenticates to get a token<br>• **Policies** — Vault's own HCL-based least-privilege access-control language<br>• **Seal/unseal** — data is encrypted at rest and requires key shares to unseal on startup |
| AWS IAM policy authoring | **Not observed** | No AWS IAM work seen | AWS's identity/access-control system.<br>• **Users/Roles/Groups** — the identities<br>• **Policies** — JSON documents defining allow/deny on specific actions + resources<br>• **Trust policies** — who/what is allowed to assume a given role<br>• Design goal: least privilege — same principle as the Headscale ACL work, different enforcement layer |
| AWS / GCP / Azure | **Not observed** | Everything I saw was DigitalOcean + self-hosted bare metal | Managed cloud platforms — compute/networking/identity/storage as API-driven services rather than something you rack and configure yourself. No hands-on evidence from this session either way. |
| Hetzner | **Unconfirmed, even in your own material** | You raised this exact question about your own infra a few days ago — searched a live node list for it and found none | (Cloud/VPS provider — no components to describe; flagged here only because it's a specific claim worth not overstating.) |

Don't read "not observed" as "you're lying on your resume" — I only know what I watched.
If any of these are real from work outside what I've seen, use that real experience and
ignore my caveat entirely. If they're genuinely thin, §4 has honest language for exactly
that situation.

---

## 3. Your five strongest stories — have these cold

These are real, specific, and will hold up under any depth of follow-up because they
happened, in detail, and I watched the actual debugging. Pick 3–4 of these depending on
how the conversation goes; don't force all five in.

**① Secrets management — this is the one Anower's post makes almost certain to come up**
- *Situation:* A Helm-chart-managed service's secrets were regenerating on every GitOps
  sync, causing pods to restart on every merge for no real reason.
- *Task:* Find why, without breaking the "everything lives in Git, encrypted" model.
- *Action:* Traced it to Helm's `lookup` function returning empty under `helm template`
  (no persisted release state to check against) — the chart's own logic for "reuse
  existing secret or generate a new one" always took the generate-new path. Fixed by
  pinning the actual secret value into a SOPS-encrypted file instead of letting the
  chart auto-generate it, so it stops changing every render.
- *Result:* Pods stopped restarting unnecessarily. Built a broader pattern on top of it
  afterward — a `needs-hash` annotation on secrets that content-hashes the secret name
  so a genuine credential rotation *does* correctly roll the pods that depend on it,
  automatically, without anyone manually bouncing anything.
- *Why this lands with Anower specifically:* it's not "we use SOPS" as a buzzword — it's
  a real root cause in how a specific tool's specific function behaves differently in
  two different execution contexts, plus a designed mechanism for the exact tradeoff his
  post is about (secrets need to be both static-and-safe *and* rotatable).

**② A production incident where your first hypothesis was wrong, and you caught it**
- *Situation:* A newly-deployed internal admin console's login kept failing with no
  useful server-side error.
- *Task:* Root-cause it without guessing.
- *Action:* First hypothesis (wrong-backend routing) looked plausible off a raw curl
  response — checked it against the actual Ingress routing (`kubectl describe ingress`)
  and it was disproven immediately. Ruled out clock skew next by comparing the pod's own
  clock against the signed request's timestamp directly. Bumped log verbosity to
  actually see the rejection reason — discovered along the way that a live `kubectl set
  env` change wasn't sticking because the GitOps controller's self-heal was reverting it
  before the new pod even finished rolling out, so had to commit the change instead of
  patching live. Eventually traced it to environment variables that had been dropped
  during a service migration, based on unverified documentation claiming they were
  deprecated — restored them, confirmed via a real login attempt.
- *Result:* Fixed, verified end to end.
- *Why this is worth telling instead of a clean-sounding fake story:* it shows real
  debugging discipline — willing to say "my first theory was wrong" out loud, verifying
  against real evidence at every step instead of pattern-matching. That's a stronger
  signal to a veteran infra person than a story where you were right immediately.

**③ Resource/capacity crisis with a real number**
- *Situation:* A private container registry hit ~29GB of a 40GB quota and kept growing.
- *Task:* Bring it under control without deleting anything actually needed (including
  keeping `latest` tags live, a real constraint).
- *Action:* Distinguished stale-UI-cache numbers from ground truth by hitting the
  registry's own quota API directly. Configured tag-retention rules (keep the last N
  pushed artifacts per repo, which — important nuance — counts by unique manifest, not
  by tag, so multiple tags pointing at the same image don't multiply the count) and ran
  garbage collection, which is a separate, required step from retention alone.
- *Result:* Real usage confirmed at 3.36GB afterward, via the API, not the dashboard.
- *Why it lands:* concrete before/after number, and the "retention alone doesn't reclaim
  disk, GC does" distinction is the kind of detail that separates "read about it" from
  "did it."

**④ Access-control / network segmentation design, iterated and self-corrected**
- *Situation:* Designing least-privilege network access rules for a private mesh
  network (Tailscale/Headscale) spanning multiple providers.
- *Task:* Default-deny, one identity gets broad access, everything else scoped tightly.
- *Action:* Went through several real iterations of the ACL policy, catching your own
  mistakes each time — a tag name that didn't describe what the rule actually granted, a
  missing rule that would've locked out every regular user with no fallback access, and
  (independently) caught a `--write-kubeconfig-mode=644` flag in a real systemd unit that
  would've made a full cluster-admin credential file world-readable on the host — pointing
  out that no network ACL matters once someone already has admin credentials sitting in a
  readable file.
- *Result:* A verified, working policy, cross-checked against the actual running
  coordinator server's live node list rather than just reasoning about it on paper.
- *Why it lands:* this is the *exact* discipline Anower's post is describing — habits and
  automation over budget, verify against the real system rather than trust the design on
  paper.

**⑤ GitOps health-check false positive + a genuinely funny/human root cause**
- *Situation:* Every application in the deployment dashboard showed "Progressing"
  (never "Healthy"), even for apps that were working fine.
- *Task:* Find out if it's a real health problem or a monitoring blind spot.
- *Action:* Root-caused the dashboard behavior to a Lua-based health check expecting a
  load-balancer status field that a NodePort-based ingress controller never populates —
  wrote a health-check override for that resource type. Separately, one specific pod
  really was crash-looping, and it turned out to be a stray, unrelated container running
  directly on the host (outside Kubernetes entirely) squatting on a port a legitimate
  monitoring exporter needed.
- *Result:* Dashboard now accurately reflects real health; false-positive noise
  eliminated.
- *Why it lands:* distinguishes "the monitoring tool is lying" from "the system is
  actually unhealthy" — a core SRE skill, and shows you don't stop investigating once
  you've fixed the first thing you find.

---

## 4. Exact language for the real gaps — don't get caught improvising

If any of these come up and you don't have real experience to substitute, use this
phrasing. It's honest, doesn't apologize, and bridges to what's actually true.

**Vault / secrets tooling generally (very likely to come up given his post):**
> "My hands-on secrets experience is SOPS with age encryption, GitOps-native —
> everything encrypted in Git, decrypted at apply-time, with a pattern for making
> rotations actually roll the workloads that depend on them. I haven't operated Vault
> day to day, but the underlying discipline — nothing in plaintext, rotation doesn't
> silently drift, least privilege on who can decrypt what — is the same problem, just a
> different tool for pulling secrets at runtime instead of baking them in at apply-time."

**Istio / service mesh:**
> "I've worked with Traefik as an ingress/reverse-proxy layer in production, including
> debugging real header-handling and auth-collision bugs at that layer. I haven't run
> Istio specifically — the concepts (mTLS between services, traffic policy, observability
> at the mesh layer) I understand, but I'd want to be upfront that my hands-on time is
> with a simpler ingress model, not a full service mesh."

**OpenTelemetry / Datadog:**
> "My production observability stack has been Prometheus, Grafana, and Loki — including
> writing custom health-check logic and root-causing false-positive alerts, not just
> looking at dashboards someone else built. I haven't operated OpenTelemetry or Datadog
> specifically; the concepts (traces/metrics/logs, alerting on signal not noise) transfer,
> but I'd rather say that directly than imply hands-on depth I don't have."

**AWS IAM / GCP / Azure:**
> "My production infrastructure has been self-hosted — bare-metal and VPS-based
> Kubernetes, not a managed cloud control plane. So I don't have AWS IAM or Azure
> RBAC production experience specifically. What I do have is real least-privilege
> access-control design experience — network-level, via Tailscale/Headscale ACLs — which
> is the same discipline, just enforced at a different layer than IAM policies."

**Terraform:**
> "The infrastructure I've provisioned has mostly been bootstrapped directly — shell
> scripts, cloud-init, manual VPS setup — rather than through Terraform modules. If the
> role needs Terraform specifically, that's a real gap I'd want to close quickly rather
> than overstate now."

Say these plainly, once, and move on. Don't re-litigate them if they come back up —
repeating the caveat reads as insecurity, not honesty.

---

## 5. Questions likely to actually get asked, given who's asking

1. **"Walk me through your secrets management setup end to end."** — Lead with story ①.
   Have the actual mechanism ready (SOPS + age, git-native, the `lookup`-function root
   cause, the rotation-triggers-rollout pattern) — not "we use SOPS," the *why* behind
   each piece.
2. **"Tell me about a time your monitoring told you the wrong thing."** — Story ⑤. Anower
   has run Wazuh/ELK/Prometheus/Grafana at real scale; he'll recognize a genuine
   false-positive story versus a rehearsed one immediately.
3. **"How do you think about resilience — what's your philosophy when something breaks?"**
   — This maps directly to Cloud Omnium's own public "built for reality, not the best
   case" framing. Answer from story ②: you don't defend your first hypothesis, you go
   find out what's actually true, even if that means admitting the first theory was wrong.
4. **"What's your experience with OpenStack / Ceph / bare-metal private cloud?"** — Be
   honest: your real depth is self-hosted K3s + a private mesh network, not
   OpenStack/Ceph specifically. Don't reach for false equivalence — say what you
   actually ran, and that the operational discipline (capacity management, GC, quota
   awareness — story ③) is the transferable part, not the specific tool.
5. **"You've moved jobs every 1-2 years — walk me through why."** — Have a clean, factual
   answer ready (each move was a real step up in scope), not a defensive one.
6. **Udayan is more likely to ask "how would you explain this tradeoff to a
   non-technical stakeholder"-style questions** — have one of the five stories ready to
   retell in plain terms, no jargon, focused on business impact (uptime, cost, risk),
   not mechanism.

---

## 6. Questions to ask them

Tailored to what they've actually said publicly, not generic:

- "Anower, you mentioned secrets management being foundational rather than a feature —
  what does that actually look like operationally at Cloud Omnium? Vault, something
  homegrown, SOPS-style GitOps?"
- "How much of the COPCS stack is genuinely multi-tenant/sovereign-compliance-driven
  versus standard OpenStack — what's the hardest part of that compliance angle in
  practice?"
- "What does the GitOps pipeline actually look like day to day — ArgoCD-style
  pull-based, or something more push-driven with Ansible/Terraform doing the apply?"
- "Where does DevOps sit relative to the OpenStack/Ceph platform team — is this role
  building the private-cloud platform itself, or building CI/CD and delivery on top of
  an already-stood-up platform?"

---

## 7. Night-before checklist

- Re-read stories ① through ⑤ once, out loud, not just silently.
- Decide now which 3-4 you'll lead with if only asked one open-ended "tell me about your
  experience" question — don't decide live under pressure.
- Have the §4 gap language ready verbatim for Vault and Istio specifically — those two
  are the most likely to come up given Anower's stack and your resume both naming them.
- Don't over-rehearse to the point of sounding scripted — these are real events, tell
  them like real events, including the parts where you were initially wrong.
