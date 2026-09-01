# More Prep — How Loki, Prometheus & Grafana Actually Work Together

Not just names — this is the "explain the architecture" version, in case Anower asks
"how does your observability stack actually fit together" instead of "what tools do you
use."

---

## The core split: two data stores, one shared viewer

Prometheus and Loki are the same idea applied to two different kinds of signal —
Prometheus for *numbers over time* (metrics), Loki for *lines of text* (logs). **Grafana
stores none of that data itself** — it's the window you look at both through, not a
data store in its own right.

---

## 1. The metrics pipeline

- **node-exporter** (a DaemonSet — one copy per node) doesn't send anything anywhere on
  its own. It just exposes an HTTP endpoint (`/metrics`) with the host's raw stats (CPU,
  memory, disk, network) sitting there, waiting to be read.
- **Prometheus** does the reaching-out. On a timer, it *scrapes* (pulls) every target
  it's configured to watch — node-exporter on every node, plus any app that exposes its
  own `/metrics`. Each data point gets stored as a labeled time series in Prometheus's
  own on-disk database (its TSDB). It also continuously evaluates alerting/recording
  rules against that stored data.
- This is a **pull model** — the exporter is passive, Prometheus is the one doing the
  fetching. If asked "how does Prometheus find out about a new metric," the answer is:
  it doesn't get pushed to, it goes and asks, on a schedule.

## 2. The logs pipeline

- **Promtail** (also a DaemonSet, one per node) tails container stdout/stderr on that
  node, tags each line with Kubernetes-aware labels (namespace, pod, container, node),
  and *pushes* those lines outward to Loki.
- **Loki** is the opposite model from Prometheus in that one specific way — it's
  push-based on the ingest side. What makes Loki cheap is that it only indexes the
  **labels**, not the full text of every log line — it groups logs into label-defined
  "streams" and stores the raw text compressed. A search is really "find streams
  matching these labels, then scan the compressed text for the time range you asked
  about," not a full-text index the way Elasticsearch works.
- Loki's query language (**LogQL**) is deliberately built to look and feel like
  Prometheus's **PromQL** — same label-matching syntax — so once you know one, the other
  isn't a new mental model, just a different payload.

## 3. Grafana — the one place you actually look

Grafana is configured with both Prometheus and Loki as "data sources" and queries them
on demand when a dashboard is open. The actual payoff of running metrics and logs side
by side: one dashboard panel shows a CPU spike from Prometheus, the panel next to it
shows the actual log lines from Loki for that same pod, same time window — you see
*that something happened* and *why* in one screen instead of switching tools.

**Honest gap to flag for yourself:** never directly saw an Alertmanager deployment or
Grafana-native alert rules in this repo. If asked "where do your alerts actually route
to," don't invent a Slack/PagerDuty integration that hasn't been verified — say plainly
that alert-routing specifics aren't something to claim with certainty, but know the two
possible paths (Prometheus rule → Alertmanager, or Grafana's own native alerting) and
that which one's in play is worth confirming before claiming it.

---

## A real example that ties the whole chain together

Worth having ready as a concrete, walk-it-out-loud story:

`node-exporter` went into `CrashLoopBackOff` because a stray container running directly
on the host — outside Kubernetes entirely — was already squatting on port 9100, the
exact port node-exporter needs to bind.

Walk the chain: if node-exporter can't start → Prometheus's scrape of that node shows
the target as *down* → Grafana's host-metrics dashboard shows a *gap* for that node, not
an error. Three completely different systems, one root cause — and tracing it backward
from "the dashboard looks wrong" to "a Docker container on the host is holding a port"
requires knowing each layer's actual job, not just its name.

---

# What Cloud Omnium's OpenStack + Ceph post is actually describing

This isn't abstract marketing language — it's describing Cloud Omnium's own real stack
(matches Anower's own "About" section: *"architecting fault-tolerant OpenStack private
cloud ecosystems (COPCS), Ceph distributed storage"* almost word for word). This is
extremely likely to come up given who's interviewing. **Important honesty note: you have
no hands-on OpenStack/Ceph experience from anything I've observed — the goal here isn't
to fake depth, it's to be able to discuss the architecture intelligently if it comes up,
and be upfront that your own storage/orchestration experience is at a much smaller scale
(self-hosted K3s + local-path storage), not an equivalent.**

## OpenStack — "the cloud control layer"

OpenStack itself doesn't run VMs or store data directly — it's the API/orchestration
layer that lets someone say "give me a VM with 4 vCPUs, attach a 100GB volume, put it on
this network" and have that actually happen across a pool of physical servers. Its core
pieces:

- **Keystone** — identity/auth; who's allowed to do what, across every other service
- **Nova** — compute; manages VM lifecycle (schedule, start, stop) across hypervisor hosts
- **Neutron** — networking; virtual networks, routers, floating IPs, security groups
- **Cinder** — block storage service; provisions the persistent volumes VMs attach to —
  **this is where Ceph plugs in**, Cinder doesn't store the bytes itself
- **Glance** — image catalog; the VM disk images used to boot new instances — also
  commonly backed by Ceph
- **Manila** — shared file storage (NFS/CIFS-style) — commonly backed by CephFS
- **Swift** — OpenStack's own native object storage — in practice, many deployments use
  Ceph's object gateway *instead of* Swift itself (see below)
- **Horizon** — the web dashboard tying it all together

## Ceph — "flexible software-defined storage across block, file, and object"

Ceph is one underlying distributed storage cluster that can serve three completely
different storage paradigms from the *same pool of physical disks* — that's the actual
meaning of "flexible... across block, file, and object" in the post, not three separate
systems.

- **RADOS** — the underlying distributed object store everything else is built on
- **OSD** (one per physical disk) — actually stores data, handles its own
  replication/recovery
- **MON** (Monitor) — maintains the cluster map (which OSDs exist, where data lives) —
  the coordination/source-of-truth service
- **CRUSH algorithm** — the distinctive part: computes *where* any piece of data should
  live across the cluster mathematically, without needing a centralized lookup table —
  worth naming specifically if asked "how does Ceph know where data is"
- Three access layers on top of the same RADOS cluster:
  - **RBD** (block device) — a virtual disk, presented to VMs — backs OpenStack's Cinder
    and Glance
  - **CephFS** — a POSIX-compliant distributed filesystem (needs its own **MDS**,
    metadata server, component) — backs OpenStack's Manila
  - **RGW** (RADOS Gateway) — an HTTP object-storage gateway, S3- and Swift-API
    compatible — can literally stand in for OpenStack's own native Swift service

## How they're actually "harnessed" together

OpenStack's compute/orchestration services don't implement their own durable storage —
each one calls out to Ceph through a specific driver: Cinder → RBD, Glance → RBD,
Manila → CephFS, object access → RGW. One Ceph cluster, one CRUSH map, simultaneously
serving VM disks, VM boot images, shared filesystem mounts, and S3-style buckets. This
combination ("OpenStack-on-Ceph") is a well-established, extremely common reference
architecture for building an entire private cloud from open-source components — no
proprietary SAN, no proprietary hypervisor licensing, no vendor-specific cloud API. That's
the real substance behind "real infrastructure control starts with technology you can
understand, adapt, and own" — it's not a slogan, it's describing a specific, standard,
nameable architecture.

---

# OpenStack and Ceph, in depth

Same honesty rule as above: this is for being able to *discuss* the architecture, not to
imply hands-on production experience you don't have. Where something maps cleanly onto
something you've genuinely run (K3s, Prometheus, etcd-style quorum stores), it's called
out explicitly — those are honest bridges, not equivalences. Say "the same idea shows up
in X, which I have run" rather than implying you've run OpenStack/Ceph themselves.

## OpenStack

**What it actually is:** not one program — a collection of independent, separately
governed open-source projects (mostly Python) that together provide IaaS: an API you
call to get a VM, a network, a volume, an image, the same way you'd call a public cloud's
API, but running on your own hardware. Originated as a joint Rackspace/NASA project
(2010), now stewarded by the Open Infrastructure Foundation.

**How it's actually deployed:** this is the part people underestimate — "installing
OpenStack" means orchestrating dozens of independent services that share a message queue
(almost always RabbitMQ) for internal async communication and a shared relational
database (MySQL/MariaDB) for each service's state. Nobody hand-installs this piece by
piece in production; it's done via a deployment tool — **Kolla-Ansible** (containerized,
Ansible-driven — genuinely relevant to your own Ansible background, worth mentioning),
**OpenStack-Ansible**, **TripleO**, or a vendor's own distribution (Cloud Omnium's own
"COPCS" is exactly this kind of packaged distribution).

**The repeating internal pattern**, clearest in Nova (compute):
`nova-api` (receives the request) → message queue → `nova-scheduler` (decides which
physical host) → `nova-compute` (runs on every hypervisor host, actually talks to
libvirt/KVM to create the VM). Every other service — Cinder, Neutron, Glance — follows
roughly this same api/scheduler/agent shape.

**Services beyond the storage-facing ones already covered:**
- **Keystone** — identity/auth, and the multi-tenancy boundary: "projects" (formerly
  "tenants") scope quotas, RBAC, and network isolation
- **Neutron** — software-defined networking; supports **provider networks** (VLAN,
  mapped straight onto physical switching) and **self-service networks** (an overlay,
  typically VXLAN or Geneve) — *this overlay concept is the exact same idea as Flannel's
  VXLAN backend riding on top of `tailscale0` in your own K3s setup, just at a different
  layer.* Neutron routers provide L3 + floating IPs (NAT giving a private VM a
  routable address) — conceptually similar to what a Kubernetes Ingress/Service does for
  pods.
- **Heat** — declarative orchestration templates (OpenStack's CloudFormation-equivalent)
- **Octavia** — Load-Balancer-as-a-Service
- **Ironic** — bare-metal-as-a-service (provisions physical machines through the same
  API used for VMs)
- **Magnum** — Container-orchestration-as-a-Service; literally provisions **Kubernetes
  clusters** through the OpenStack API. Worth knowing this exists specifically because it
  ties OpenStack directly back to something you do have real depth in.

**Known-hard operational realities** (mentioning these signals real understanding, not
just reading the marketing page): control-plane services need to run HA (3+ controller
nodes behind a load balancer for Keystone/Nova-api/Neutron-server); the shared message
queue is a common bottleneck/failure point at scale; **upgrades are notoriously
difficult** because of tight interdependencies between service versions — this is one of
OpenStack's most well-known real pain points, not a hidden detail; very large Nova
deployments split into **cells** so one giant shared DB/queue doesn't become a
bottleneck.

## Ceph

**What it actually is:** a unified, distributed storage system designed from the start to
have no single point of failure and to scale by adding commodity hardware rather than
buying bigger specialized boxes. Originated as Sage Weil's PhD project at UC Santa Cruz;
stewarded commercially through Inktank → Red Hat → now IBM.

**The core indirection that makes it work — object → PG → OSD, not object → OSD
directly:**
- **RADOS** — the underlying store; regardless of which access layer you use (RBD,
  CephFS, RGW), everything ultimately becomes an object in RADOS.
- **OSD** (Object Storage Daemon) — one per physical disk; stores data, replicates to
  peers, self-heals on failure.
- **PG (Placement Group)** — objects are first hashed into a fixed number of PGs; only
  *then* does CRUSH map PGs onto OSDs. This indirection is why adding/removing an OSD
  only reshuffles PG→OSD mappings, not every individual object — that's what makes
  rebalancing tractable at scale.
- **CRUSH** — the deterministic placement algorithm, driven by a **CRUSH map** describing
  physical topology (which OSD is in which host/rack/datacenter). This is what lets you
  say "always replicate across racks, never put two replicas on the same server" without
  a centralized lookup table for every object.
- **MON (Monitor)** — holds the cluster's maps (OSD map, MON map, PG map, CRUSH map),
  runs as a small odd-numbered quorum (typically 3 or 5). *This is architecturally the
  same shape as etcd's role in Kubernetes — a small quorum-based store holding the
  cluster's source of truth — a genuinely honest bridge, since you do understand etcd's
  role from real K8s work.*
- **MGR (Manager)** — runs alongside MONs; handles metrics, the Ceph dashboard, and
  orchestrator integrations. **Ceph ships a built-in Prometheus-exporter mgr module** —
  worth knowing this specifically, since it's a real, honest connection to observability
  work you've actually done, not a stretch.
- **Pools** — logical partitions of the cluster, each with its own durability strategy:
  **replication** (N full copies — simple, higher storage overhead, e.g. 3x) or
  **erasure coding** (RAID6-style parity across chunks — better storage efficiency, more
  CPU/network cost on recovery). Knowing this tradeoff exists is worth more than naming
  either term alone.

**The three access layers, one detail deeper than before:**
- **RBD** — a virtual block device; supports snapshots and copy-on-write clones, thin
  provisioning.
- **CephFS** — needs one more component beyond OSDs/MONs: the **MDS** (Metadata Server),
  which handles POSIX filesystem metadata (directory structure, file attributes)
  separately from the actual file data, which still lives in RADOS. Can run
  active/standby or multiple active MDS for scale.
- **RGW (RADOS Gateway)** — the HTTP daemon providing S3- and Swift-compatible APIs; this
  is how object storage is actually consumed day to day, not by talking to RADOS
  directly.

**Known-hard operational realities:** OSD failure triggers automatic self-healing, but
that recovery/backfill generates real I/O load on the cluster — a genuine operational
concern, not free; production deployments commonly separate a **cluster network** (OSD-
to-OSD replication traffic) from a **public network** (client traffic) so recovery
doesn't starve client I/O; minimum viable cluster sizing (odd-numbered MON quorum,
meaningful OSD counts for real redundancy) is a real design decision, not an
afterthought.

---

# Self-healing and auto-scaling — how it's actually achieved

Another Cloud Omnium-advertised capability, worth the same treatment: not a single
product, but a stack of well-known mechanisms layered on top of each other. Same honesty
rule — you have real, direct experience with the Kubernetes-level version of this (the
ArgoCD `selfHeal` mechanism, K8s liveness/readiness probes, HPA-style concepts), and no
verified experience with the OpenStack/Ceph-level equivalents. Both are described below
so you can speak to the *pattern* honestly regardless of which layer the question lands
on.

## The general shape, at any layer

Self-healing and auto-scaling are really the same idea applied twice: a control loop
that continuously compares **desired state** against **observed/actual state**, and takes
action to close the gap. Every system below is a variation of that one loop —
*observe → compare → act → repeat.* If asked "how would you design this," that loop is
the answer, then you fill in what's doing the observing and what action gets taken.

## At the layer you've actually worked in — Kubernetes

- **Liveness/readiness probes** — kubelet periodically checks each container; a failed
  liveness probe gets the container restarted, a failed readiness probe pulls the pod
  out of Service load-balancing without restarting it. This is the smallest, fastest
  self-healing loop — no scheduler involved, purely local to the node.
- **Deployment/ReplicaSet controllers** — the reconciliation loop you already know from
  your own real work: if a pod dies for any reason, the controller notices the actual
  replica count doesn't match desired, and creates a replacement, on whichever healthy
  node the scheduler picks.
- **ArgoCD's `selfHeal: true`** — the same reconciliation idea one layer up, at the
  *desired state* level itself: if live cluster state drifts from what's declared in Git
  (someone runs `kubectl edit` by hand), ArgoCD reverts it back to match Git — you've
  hit this directly and had a live fix get reverted by it.
- **HPA (Horizontal Pod Autoscaler)** — the auto-scaling half: watches a metric
  (typically CPU/memory, or a custom metric via the Prometheus adapter) and adjusts
  replica count up or down to match a target. This is the Kubernetes-native version of
  exactly what the Cloud Omnium line is describing — "automated monitoring... triggers
  ... resource scaling without manual intervention" is a near-literal description of
  what HPA does.
- **Cluster Autoscaler** — one level below HPA: if pods can't be scheduled because no
  node has room, it adds a node; if nodes sit underused, it removes one. HPA scales pod
  *count*, Cluster Autoscaler scales the *node pool* underneath it.

## At the OpenStack/Ceph layer — the equivalent mechanisms, not personally verified

- **Ceph self-healing**: already covered above — an OSD failing triggers automatic
  re-replication of the PGs it held, driven by MONs noticing the OSD map changed and
  CRUSH recomputing placement. This is the same observe→compare→act loop, just for disk
  failures instead of pod failures.
- **Nova/Neutron failure recovery**: OpenStack itself doesn't auto-heal a failed VM the
  way Kubernetes does by default (a crashed VM doesn't automatically get recreated unless
  something else is watching it) — this is a real, meaningful difference from Kubernetes
  worth knowing rather than assuming symmetry. That "something else" is usually:
- **Watcher** — OpenStack's own resource-optimization service: monitors telemetry
  (via Ceilometer/Gnocchi or an external system like Prometheus) and can trigger actions
  like live-migrating a VM off an overloaded host, or evacuating VMs off a host it
  detects as failed.
- **Heat autoscaling** — Heat (the orchestration service) supports autoscaling groups
  conceptually similar to AWS Auto Scaling Groups: a scaling policy tied to a Ceilometer
  alarm, adding/removing VM instances from a group when a metric crosses a threshold.
- **External monitoring closing the loop**: in practice, most real self-healing/
  auto-scaling in an OpenStack environment is wired through the same
  Prometheus/Grafana/Alertmanager stack already covered above — Prometheus fires an
  alert, and either a human runbook or an automated action (via Watcher, a Heat
  autoscaling policy, or a custom operator script) is what actually executes the
  response. **This is worth saying explicitly if asked**: "self-healing" isn't one
  button Cloud Omnium flips on — it's the observability stack (which you do have real
  depth in) wired up to trigger the orchestration layer's remediation actions (which you'd
  be honest about not having personally configured at the OpenStack level).

---

# Security & Isolation — short answers only

Deliberately kept to 2-3 sentences each — this is "I know the term and the shape of it,"
not a deep-dive. Say the short version, don't pad it out to sound like more than it is.

**Encryption at Rest**
> Data sitting on disk is encrypted, not just protected by access controls — so a stolen
> or decommissioned drive is unreadable without the key. Ceph does this per-OSD via
> dm-crypt/LUKS. I haven't configured that specifically, but it's the same principle as
> what I do with SOPS+age for secrets — nothing sensitive ever sits in plaintext, just
> enforced at a different layer (files in Git vs. blocks on disk).

**Encryption in Transit**
> Data is encrypted while it's actually moving across the network, not just at rest —
> TLS between clients and APIs, often mTLS or IPsec between internal control-plane or
> storage nodes. I have real, direct experience here at the edge — Caddy's automatic TLS
> termination, and I've found and fixed two separate real bugs specifically around how
> auth headers behave inside an encrypted proxy chain.

**Multi-Tenant Data Isolation**
> Different customers share the same physical hardware but should never be able to see
> or reach each other's data or traffic — enforced in OpenStack via Keystone project
> boundaries plus Neutron's per-tenant network overlays, and in Ceph via separate pools
> or namespaces. I haven't built that specific boundary, but it's the same isolation
> principle as the least-privilege Tailscale/Headscale ACL work I have done — one
> identity's devices provably can't reach another's, just enforced at a different layer.

---

# Cloud-Native Integration — short answers only

Same rule: 2-3 sentences, say the real bridge, don't inflate it.

**Ceph-Based Software-Defined Architecture**
> "Software-defined" means the storage layer is entirely config-driven — pools, replication
> rules, and placement (CRUSH) are defined in software, not tied to a specific vendor's SAN
> appliance, so it runs on commodity hardware and reconfigures without a hardware swap.
> That's the same "infrastructure as config, not manual setup" idea behind my own Kustomize/
> GitOps work — different domain, same underlying philosophy.

**OpenStack Integration**
> Ceph isn't bolted on as an afterthought — it *is* OpenStack's storage backend, plugged in
> through each service's own driver (Cinder → RBD for volumes, Glance → RBD for images,
> Manila → CephFS for shares), so one Ceph cluster serves the whole compute layer. I
> haven't configured those drivers myself, but I've done the equivalent GitOps-side
> integration work — wiring a storage backend (RustFS, S3-compatible) into an existing
> deployment pipeline via its own PVC/StorageClass.

**Kubernetes Integration**
> Ceph ships its own Kubernetes CSI (Container Storage Interface) driver, so a
> PersistentVolumeClaim can be dynamically provisioned straight onto RBD (block) or CephFS
> (shared filesystem) — the same PVC/StorageClass mechanism I've actually used, just backed
> by `local-path` in my case instead of Ceph. That's a genuinely direct, honest bridge: same
> Kubernetes-side abstraction, different storage backend underneath it.

---

# How they actually manage a multi-tenant storage cluster

Note up front: this is inferred from their public page plus standard, well-documented
Ceph/OpenStack operating patterns — not insider knowledge of Cloud Omnium's specific
implementation. Say it that way if asked: "based on the standard pattern for a stack
like that" rather than implying you know their internals.

Their listed tech stack is **Cinder, MinIO, Ceph, Swift, Kubernetes** — notably *not*
just "Ceph" alone. That combination tells you something real: Ceph is the actual
durable storage substrate underneath everything, and Cinder/Swift/MinIO are the
different **provisioning/API faces** put in front of it depending on what a customer
actually wants (a block volume, an S3 bucket, etc.) — same pattern already covered
above, just with one more piece: **MinIO is a separate, lightweight, single-binary
S3-compatible object store** — functionally the closest thing on their whole stack to
**RustFS, which you've actually deployed and operated yourself**. That's a genuinely
direct bridge, not a stretch — same category of system, same S3 API surface, you've
just run the other implementation of the same idea.

## The actual day-2 management questions, and how they're really answered

**"How do you give a tenant storage without them touching another tenant's data?"**
Ceph's own auth system (`cephx`) issues capability-scoped keys — a key can be
restricted to a specific pool or namespace, so a tenant's client literally cannot
authenticate against another tenant's pool. Layered under OpenStack, this maps onto
Keystone project boundaries (Cinder volumes and Swift containers are scoped per
project). The "management" part isn't manual per-customer configuration at scale — it's
this scoping being applied consistently by whatever provisioning automation sits in
front of it.

**"How do you know when you're about to run out of space, before it becomes an
incident?"** This is their own "Capacity Planning & Forecasting" bullet, and it's the
*exact* skill in your real Harbor quota story — distinguishing a stale cached number
from ground truth (their dashboard vs. the registry's own quota API), then actually
reclaiming space (tag retention + garbage collection) rather than just watching the
number climb. Same discipline, different storage system: alert on real utilization
trend *before* the hard limit, not after.

**"How do the storage tiers (Performance/Standard/Archive) actually work
mechanically?"** Ceph supports tagging OSDs by **device class** (`hdd`, `ssd`, `nvme`)
and a CRUSH rule can target a specific device class — so a "tier" isn't a separate
product, it's a separate pool whose CRUSH rule only places data on OSDs of one device
class. "Auto-migration" between tiers is typically a lifecycle policy (age-based or
access-pattern-based) that copies/moves objects from one pool to another automatically —
conceptually identical to S3 lifecycle rules if you've seen those.

**"What actually happens when a node dies?"** — covered above in the Ceph section:
MONs notice the OSD map changed, CRUSH recomputes placement for the PGs that node held,
and re-replication starts automatically. The *management* side of this is knowing
cluster health state (`HEALTH_OK` / `HEALTH_WARN` / `HEALTH_ERR`, and *why* — degraded
PGs, nearfull OSDs, etc.) rather than treating "the dashboard is red" as the whole
diagnosis — the same "don't stop at the first symptom" discipline from your ArgoCD
health-check / node-exporter story.

**"How is all of this actually monitored day to day?"** Ceph's `ceph-mgr` ships a
built-in Prometheus exporter module (already noted earlier) — so in practice, cluster
health, capacity, and PG state all become Prometheus metrics, visualized in Grafana,
alongside Ceph's own built-in dashboard for cluster-wide operational visibility. This is
the one place your real, demonstrated depth (Prometheus/Grafana, real alert
false-positive debugging) transfers almost directly, regardless of what's underneath.

## The honest, DevOps-shaped answer, if asked directly

Don't try to answer "how would you manage our storage cluster" as if you're a Ceph
storage engineer — you'd lose that comparison to Anower instantly, and he'd know it. The
credible DevOps answer is: *bring the same discipline you've already proven, to this
storage system specifically* —

> "I haven't operated a Ceph cluster day to day, so I'd want to be upfront about that.
> What I would bring is the same approach I've used managing storage capacity elsewhere —
> distinguishing real usage from what a dashboard shows, catching capacity trends before
> they become incidents, and making provisioning/quota changes go through a declarative,
> auditable pipeline rather than manual one-off configuration — plus wiring health and
> capacity signals through Prometheus/Grafana, which is where I already have real depth.
> The tool underneath would be new to me; the operational discipline wouldn't be."

---

# Their SIEM solution — what's actually going on

A SIEM (Security Information and Event Management) system does the same core loop as
the observability stack already covered — **ingest → normalize → correlate → alert** —
just applied to *security* events instead of operational metrics/logs, and the
"correlate" step is doing much more work: a single failed login means nothing, but
failed logins from one IP followed by a successful login followed by a new file
appearing in a sensitive directory, all within ten minutes, is a real pattern worth
alerting on. That's the actual value a SIEM adds over just having logs somewhere.

**Their stack is the classic ELK/Elastic stack, plus dedicated security tooling layered
on top:**

- **Elasticsearch** — the indexed storage layer. Genuinely worth naming the contrast
  with something you do know: **Loki deliberately avoids full-text indexing to stay
  cheap** (indexes only labels); **Elasticsearch takes the opposite tradeoff** — it
  fully indexes document content via Lucene, which costs more but is exactly why
  arbitrary full-text security search/correlation across huge event volumes is fast.
  Same problem space, opposite design decision, for a good reason on both sides.
- **Logstash** — the ingest pipeline: input → filter (parses unstructured log lines
  into structured fields, e.g. via grok patterns) → output, typically into
  Elasticsearch. Heavier-weight, transformation-focused, versus:
- **Filebeat** — a lightweight, single-purpose per-host shipping agent. **This is
  architecturally the same role as Promtail in your own stack** — tails logs, ships
  them onward — just shipping into Logstash/Elasticsearch instead of Loki. Direct,
  honest bridge.
- **Kibana** — the visualization/dashboard layer for Elasticsearch data.
  **Architecturally Grafana's counterpart**, just scoped specifically to Elasticsearch
  rather than being data-source-agnostic. Another direct bridge.
- **Wazuh** — host-based intrusion detection (originally a fork of OSSEC): agents on
  each monitored host feed a central manager that does file-integrity monitoring
  (alerts on unauthorized file changes), configuration-drift/compliance checks,
  rootkit detection, and log-based correlation rules. **No natural bridge to anything
  you've done — be honest this is a genuinely different domain (host-level security
  agent tooling), not observability.**
- **Suricata** — network intrusion detection/prevention: inspects raw traffic at the
  packet level against signature rules, flags exploits/C2 traffic/protocol anomalies.
  **Also no natural bridge — genuinely unfamiliar territory, say so plainly if asked.**
- **MISP** (Malware Information Sharing Platform) — an open-source platform for storing
  and sharing Indicators of Compromise (known-bad IPs/hashes/domains) across
  organizations; the SIEM queries it to enrich events with "is this a known threat."
- **VirusTotal** — a hosted reputation-lookup API (aggregates dozens of AV
  engines/blocklists); given a file hash or URL, returns a reputation verdict, used the
  same enrichment way as MISP.

**Deployment models** (Managed / Dedicated / Hybrid) are a generic infra-delivery
pattern you already understand conceptually even without SIEM-specific depth — the same
shape as "fully managed vs. self-hosted vs. split" for any infrastructure service, not
something unique to security tooling.

## The honest answer, if this comes up directly

Security-tooling depth (Wazuh/Suricata/MISP-style threat detection) is genuinely outside
anything demonstrated this session — don't reach for a bridge that isn't there.

> "My real depth is on the observability half of this pattern — shipping and indexing
> logs, visualizing them, building alerting that's actually low-noise rather than just
> loud — via Prometheus/Grafana/Loki. I don't have hands-on experience with host-based
> intrusion detection or network IDS tooling specifically like Wazuh or Suricata. The
> ingest-normalize-correlate-alert pipeline shape is one I understand well from the
> observability side, so I'd expect to ramp on the security-specific rule logic faster
> than the underlying pipeline architecture, but I want to be upfront that the security
> tooling itself is new to me."

---

# RBAC in Kubernetes — grounded in your own repo, not theory

Unlike most of the sections above, this one **is real, verified, hands-on work** — four
actual RBAC manifests found in the `developer` repo (`traefik/rbac.yaml`,
`monitoring/prometheus-rbac.yaml`, `logging/promtail-daemonset.yaml`,
`monitoring/kube-state-metrics-deployment.yaml`). Lean into this one with real
confidence, not a bridge — it's Kubernetes-native least-privilege access control, the
exact same discipline as the Headscale ACL work, just enforced against the Kubernetes
API server instead of a network.

## The four building blocks

- **ServiceAccount** — an identity for a *process running in the cluster* (a pod), as
  opposed to a human user. Every pod runs as some ServiceAccount whether you set one
  explicitly or not (defaults to a namespace's `default` account if unset).
- **Role** / **ClusterRole** — just a list of permissions (`rules`): which API groups,
  which resource types, which verbs (`get`/`list`/`watch`/`create`/`update`/`patch`/
  `delete`) are allowed. **A Role/ClusterRole grants nothing by itself** — it's a
  definition, not a grant. `Role` is namespace-scoped; `ClusterRole` is cluster-wide.
- **RoleBinding** / **ClusterRoleBinding** — the thing that actually *grants* a
  Role/ClusterRole's permissions to a subject (a ServiceAccount, User, or Group). A
  `ClusterRole` can even be bound via a namespace-scoped `RoleBinding` to limit its use
  to one namespace — the permission definition and its scope of use are two separate
  decisions.

## Four real examples from your own repo, each showing a different reason for the shape it takes

**Traefik** — `ClusterRole` (not namespaced), because an ingress controller's whole job
is routing traffic for *any* app in *any* namespace, so it needs cluster-wide visibility
by nature, not by accident. Real permissions worth naming specifically: `get/list/watch`
on `services`/`endpoints`/`secrets` (needs to discover what to route to, and read TLS
certs referenced by Ingress objects); `get/list/watch` on `ingresses` across both
`extensions` and `networking.k8s.io` API groups; **`update` on the `ingresses/status`
subresource specifically** — this is the exact permission behind the earlier ArgoCD
health-check story: Traefik *is* allowed to write `status.loadBalancer`, it just doesn't
populate it meaningfully on a NodePort setup, which is why the Lua override was needed —
the RBAC permission existing doesn't mean the field gets filled in the way you'd assume;
and `get/list/watch` on Traefik's own CRDs across *both* `traefik.containo.us` and
`traefik.io` API groups (the dual-API-group detail from your actual CRD work).

**Prometheus** — two separate `ClusterRole`s. One for cluster-wide service discovery
(`get/list/watch` on `nodes`/`services`/`endpoints`/`pods` — this is what backs
Prometheus's `kubernetes_sd_configs`, dynamically discovering scrape targets by
querying the API server instead of a static target list) plus a `nonResourceURLs` rule
on `/metrics` and `/metrics/cadvisor`. **`nonResourceURLs` is worth knowing specifically**
— most RBAC rules govern typed resources like `pods`; this is the less-common form that
governs a raw HTTP path on the API server that isn't a "resource" at all. A second,
separate `ClusterRole` (`prometheus-kubelet`) scopes `nodes/metrics`, `nodes/stats`,
`nodes/proxy` specifically for reaching each node's kubelet API directly.

**Promtail** — same discovery-shaped permissions as Prometheus
(`nodes`/`services`/`endpoints`/`pods`, `get/list/watch`), because it attaches
namespace/pod/container labels to log lines by querying the API for that metadata. Worth
being precise if asked: the DaemonSet's pod also runs `privileged: true` and
`runAsUser: 0` to read `/var/log/pods/*` on the host — **that's a completely separate
access-control layer from RBAC.** RBAC governs what the ServiceAccount can do *against
the API server*; `privileged`/`runAsUser` governs what the container can do *on the
host/node*. Easy to conflate the two, worth not doing that out loud.

**kube-state-metrics** — the widest `ClusterRole` of the four: `list`/`watch` across a
huge range of types (ConfigMaps, Secrets, Nodes, Pods, PVCs, Deployments, Jobs, HPAs,
Ingresses, StorageClasses...) because its entire job is turning Kubernetes *object
state itself* into Prometheus metrics (`kube_deployment_status_replicas`,
`kube_pod_status_phase`, etc.). **The genuinely sharp detail worth having ready**: it
has `list`/`watch` on `Secrets` — which, at the raw Kubernetes API level, *does* return
full secret data, not just metadata — but kube-state-metrics's own application code
never surfaces the actual secret *values* in its output, only metadata like type/labels/
existence. **That's the real point to make if this comes up: RBAC defines what's
technically possible, not what the application actually does with that access** — least
privilege is the API permission *and* trusting the application's own restraint on top of
it. That's a genuinely deeper point than "list/watch is read-only," and it connects
directly back to your own secrets-management story from earlier in this document.

---

# Their "CI/CD Implementation" service page — the one that likely matters most

You're right to flag this one differently. Storage, SIEM, and OpenStack were adjacent —
this page describes almost exactly the shape of your actual resume title. Read it as the
probable job scope, and note where your real work lines up directly versus where the
named tools don't match what you've actually run.

## Feature-by-feature against what's actually verified

**"Custom CI/CD Pipeline Architecture... avoid generic templates that don't fit your
delivery needs"** — **This is your strongest possible answer on the entire page.** This
whole session *was* exactly this: a purpose-built Kustomize/ArgoCD pipeline for a
multi-app, multi-provider setup — not a copy-pasted template. Real, specific things to
cite: KSOPS secret generators wired into Kustomize's build step, custom ArgoCD health
checks written in Lua for a resource type that doesn't report status the default way,
the `needs-hash` annotation pattern designed to solve a real rotation problem. That's
what "architected around your tech stack, not generic rules" actually looks like in
practice — you didn't just use ArgoCD, you extended how it evaluates health and how
secrets interact with it.

**"GitOps Workflow Implementation — ArgoCD-based GitOps... 90% reduction in manual
deployment errors... full audit trail of every deployment"** — **Also a direct, strong
match.** Your real setup: `automated: {prune, selfHeal}`, an app-of-apps pattern, "a
push to the default branch is picked up and applied by ArgoCD's own sync loop directly"
— no manual `kubectl apply` in the loop at all, which is the literal mechanism behind
their "audit trail" and "reduced manual error" claims. You also have the more interesting
story most candidates won't: **you've hit `selfHeal` reverting a live manual fix before
it finished rolling out** — that's not a weakness to mention, it's proof you understand
the mechanism deeply enough to have been bitten by it and adapted (committing the change
instead of patching live).

**"Artifact Repository Management — Harbor as a Docker registry"** — **Named
specifically, and you have a real incident with real numbers.** Storage quota crisis
(29.38GiB of a 40GiB quota), root-caused via the registry's own quota API instead of
trusting a stale dashboard, resolved with tag retention (last-N-artifacts, preserving
`latest`) plus a separate garbage-collection run, confirmed down to 3.36GiB afterward.
This is about as concretely provable as any story in this whole document — lead with it
if artifact/registry management comes up.

**"Environment Provisioning — Terraform + Ansible combination, IaC"** — **Real gap, and
it's the one place your resume and your actual observed work disagree with each other.**
The real infra bootstrap I watched was manual shell/`curl`-piped install scripts and
direct `kubectl`/Kustomize — not Terraform modules. If this is genuinely central to the
role, don't let this surface as a surprise: "My IaC experience is Kustomize/Helm for the
Kubernetes layer specifically — the host-level provisioning I've done has been direct
shell/cloud-init rather than Terraform modules. I'd want to be upfront that Terraform
specifically is a gap I'd close quickly rather than overstate now."

**"Auto-Checking Code Quality — SonarQube"** — Not observed anywhere. A clean, simple
gap: "I haven't used SonarQube or a similar static-analysis gate specifically — my
quality/reliability practice has been more on the deployment-safety side (health checks,
GitOps drift prevention) than static code analysis."

**"Rollback & Recovery Automation... Rollout Strategies... blue-green and canary rollout
support"** — **Partial, be precise about the distinction.** What's real: rolling-update
discipline via readiness probes gating traffic cutover, and ArgoCD's own sync/rollback
model (reverting to a previous Git state). What's *not* verified: blue-green or canary
specifically as named, distinct deployment strategies (separate environments/traffic-split
mechanisms) — rolling updates and blue-green/canary solve the same problem (no-downtime,
safe rollout) but are mechanically different. Don't call a rolling update "blue-green" if
asked to be specific — say what you actually have and that the underlying goal (verify
health before committing traffic) is the same principle.

**"GitLab CI, Jenkins"** — named again here, same gap as everywhere else in this
document: real CI/CD experience is GitHub Actions from what's actually been observed.
Don't let this be the third time it surprises you in one interview — decide now how
you'll phrase it once and move on.

**Migration coverage — "Legacy & On-Premise Systems... existing workloads carefully
moved with controlled handling and minimal disruption"** — **A real, if modest, story
exists.** The actual VPS bootstrap documented in this repo explicitly wasn't a blank
slate — it had a pre-existing native nginx setup serving two live sites before any of
this began, and the migration plan was: bring each site's routing into the new
edge-proxy config first, *then* cut over, rather than taking the sites down to switch
tooling. Small-scale, but genuinely the same category of problem this bullet describes.

## The honest bottom line for this page specifically

If this is the actual role: your strongest material (GitOps/ArgoCD depth, Harbor with a
real quantified incident, purpose-built pipeline architecture, real debugging discipline)
maps directly onto their two headline features. Your real gaps are concentrated and
already known — Terraform, GitLab CI/Jenkins, SonarQube, and the blue-green/canary
distinction specifically. That's a much better position than it sounds: four named,
answerable gaps against a role where the *hardest* two things they're selling (custom
GitOps architecture, artifact/rollback management) are exactly where you're strongest,
not where you're weakest.

---

# Network Cutover & PXE Boot

## Network cutover — and this one is real, verified work, not a bridge

A "cutover" is the actual moment live traffic gets switched from an old environment to a
new one — everything before it (building, testing, validating the new environment) can
happen without affecting production; the cutover is the specific, risky moment real users
start hitting the new thing instead of the old one.

**The main mechanics:**
- **DNS-based cutover** — repointing a domain's A/AAAA/CNAME record from the old
  infrastructure's IP to the new one. The real gotcha is **TTL (Time To Live)**: if the
  old record has a long TTL, some resolvers keep serving the cached old IP for up to
  that long even after the record changes. Standard practice: lower the TTL well *before*
  the planned cutover (so the low value has time to propagate), do the actual switch once
  it's short, then raise it back up once the new environment is confirmed stable.
- **Minimal-downtime staging** — keep the old environment fully live while the new one is
  validated in parallel; only cut traffic over once it's proven healthy, so a revert is
  just repointing DNS back rather than rebuilding anything.
- **Staged/partial cutover** — weighted DNS or a load-balancer split sending a small
  percentage of traffic to the new environment first, increasing gradually — same
  underlying idea as a canary rollout, just applied at the infrastructure-migration level
  instead of the application-deployment level.

**This is directly, actually documented in your own `developer` repo** — not a bridge,
real work: the setup guide has an explicit DNS-cutover section noting that several app
domains still pointed at the *old* server the whole repo was migrating away from, with
the rule "repoint DNS only after the new cluster is actually live — cutting over first
just means real users hit a half-deployed cluster," plus the actual verification command
used to check current DNS state before touching anything (`dig +short <domain> A`). Also
directly connects to something else already verified: Caddy's automatic TLS issuance
only succeeds once DNS genuinely resolves to the new box — so a premature cutover doesn't
just risk downtime, it can strand the new environment without valid certificates too.

## PXE Boot — real concept, genuinely no bridge to anything observed

**PXE (Preboot eXecution Environment)** lets a physical machine boot an OS over the
network — no local drive, no pre-installed OS, no one walking around with a USB
installer. Mechanically: the machine's NIC/BIOS is configured to PXE-boot, it gets a
DHCP lease plus the location of a boot file, downloads a minimal boot image over TFTP,
and that kicks off an automated install (Kickstart/preseed/cloud-init-style). This is
exactly the mechanism underneath **OpenStack's Ironic** (bare-metal-as-a-service,
already covered earlier in this document) — Ironic uses PXE boot as its actual
provisioning method for physical machines.

**Honest note**: this is real, correctly-understood territory, but genuinely no bridge to
anything you've done — the real infra bootstrap observed this session was on already-
provisioned cloud VPS instances reached over SSH, not bare-metal PXE provisioning. Say so
plainly if it comes up: "I understand PXE conceptually — DHCP/TFTP-driven network boot
for provisioning physical machines without a local install medium — but I haven't
operated a PXE-based provisioning pipeline myself. My own bootstrap work has been on
already-running cloud VPS instances, not racking bare metal."

---

# Cloud Omnium "Zero Downtime Deployment Service" page — term by term

Same method as before: real bridges get the actual repo evidence; gaps get flagged
honestly rather than stretched. Terms already covered in depth elsewhere in this file or
in `interview_prep.md` (Kustomize, Helm, ArgoCD, SOPS, Ansible, Terraform, Jenkins,
OpenStack, blue-green/canary/rolling deploys) aren't repeated here — only what's new on
this specific page.

## FluxCD — listed alongside ArgoCD in their stack, not yet covered

FluxCD is ArgoCD's direct peer: another GitOps continuous-delivery controller that
reconciles a live cluster against a Git repo, built by Weaveworks and now a CNCF project.
Mechanically the same core idea already verified deeply against ArgoCD in this repo — a
controller diffing live state vs. Git and applying/reverting — but a different design:

- **Flux** is a set of composable controllers (`source-controller` watches the Git repo,
  `kustomize-controller` applies Kustomize output, `helm-controller` applies Helm
  releases) — no built-in UI by default (Weave GitOps / Flux's own dashboard is optional
  and separate). CLI/CRD-first.
- **ArgoCD** ships one integrated app (UI + API server + repo server + app controller)
  with the UI as a first-class citizen from day one.
- Both support the same core mechanics already verified in this repo: auto-sync,
  drift-detection/self-heal, Kustomize and Helm as native render backends, app-of-apps
  style composition (Flux calls it `Kustomization` resources referencing other
  `Kustomization` resources).

**Honest framing**: real, deep hands-on GitOps experience here is 100% ArgoCD-specific —
the actual mechanics of ArgoCD's reconcile loop, its `automated: {prune, selfHeal}`
behavior, its app-of-apps pattern, and real bugs diagnosed in it (see
`project_zalmi_gitops_arc` memory / `study/argocd.md`). Flux is the same problem solved
by a different tool never actually run: "My GitOps depth is ArgoCD specifically — app-of-
apps, sync policies, a few real bugs fixed in production. I haven't run Flux, but the
underlying model (Git as source of truth, a controller reconciling drift) is the same
one, so ramping on Flux's CRD-based approach specifically would be the new part, not the
concept."

## Deployment Approval Gates — real gap, and it cuts the *opposite* direction from most

This page's "Approval Gates" means a manual human sign-off step inserted into a pipeline
before a change reaches production — the deploy pauses and waits for someone to click
approve. This is a genuine, clean gap, worth stating plainly rather than bridging: this
repo's ArgoCD setup is the **opposite** of gated — every one of its 11 Applications runs
`automated: {prune: true, selfHeal: true}`, meaning it syncs automatically the instant
Git changes, with zero manual approval step anywhere in the path from commit to
production. That's a deliberate, real, and defensible architecture (full auto-GitOps is
a legitimate industry pattern, not a shortcut) — but it is not the same thing as an
approval-gated pipeline, and this session never built or exercised one. Honest framing:
"My real environment runs fully automated sync with no manual gate — I understand
approval gates conceptually (ArgoCD supports them via manual sync policy, or a CI stage
that pauses for a `workflow_dispatch`-style human trigger), but haven't operated one."

## Automated Smoke Testing — real gap, be honest

A smoke test is a small, fast post-deploy check ("did the app come up and respond at
all") run automatically right after a release, distinct from a full test suite —
designed to catch a completely broken deploy within seconds, before real users do.
Nothing like this exists as an automated pipeline step in anything observed here — the
closest real mechanism is Kubernetes' own `readinessProbe`/`livenessProbe` (already
covered elsewhere: gates traffic at the pod level, not the release level) and the manual
`curl -i` verification checks this session ran by hand after changes (e.g. the RustFS
plan's own verification section). Those are real but manual, ad hoc, and run by a person,
not a scripted, automated pipeline gate. Say so plainly if asked: "I haven't built an
automated smoke-test stage — my post-deploy verification here has been manual curl/kubectl
checks, plus relying on readiness probes to gate traffic. I understand the pattern (a
scripted health-check job as a required pipeline stage before marking a release
successful) but haven't automated it."

## Feature Flag Integration — real gap, be honest

Feature flags decouple *deploying* code from *releasing* a feature — the code ships to
production behind a runtime toggle (checked against a flag service like LaunchDarkly,
Flagsmith, or a simple config/DB-backed boolean), so a team can turn a feature on for a
subset of users, or roll it back instantly without a redeploy, purely by flipping the
toggle. Nothing in this repo does this — every rollout observed here is a real code
deploy (new image tag → ArgoCD sync), not a flag flip. Worth being direct about the gap
rather than reaching for a false equivalence: "I haven't implemented feature-flag-gated
releases — every release in my environment is a real deploy via image tag + ArgoCD sync,
not a flag toggle. I understand the pattern and why it matters for decoupling deploy risk
from release risk, but it's not something I've built."

## Compliance & Certifications (ITIL, ISO 27001, SOC 2) — real gap, be honest

These are process/audit frameworks, not technical mechanisms — worth knowing what each
actually is so a claim of "compliance" from an interviewer doesn't sound like a black box:

- **ITIL** — a service-management framework (incident/change/problem management
  processes) — organizational process discipline, not a piece of software.
- **ISO 27001** — an information-security-management-system standard; certification means
  an external auditor verified a documented set of security controls and processes are
  actually followed, not just written down.
- **SOC 2** — a US auditing standard (Type I: controls exist at a point in time; Type II:
  controls were followed *over a period*, usually 6–12 months) built around five "trust
  service criteria" (security, availability, processing integrity, confidentiality,
  privacy) — common in vendor security questionnaires for SaaS companies.

**Honest framing**: nothing in this session's work has touched a compliance audit
process — SOPS-encrypted secrets and least-privilege patterns already in this repo are
the *kind* of control an ISO 27001/SOC 2 audit would check for, but implementing a
control isn't the same as having gone through a certification audit. Say plainly: "I've
built the technical controls these frameworks care about — encrypted secrets, access
separation — but haven't been through a formal ISO 27001/SOC 2 audit process myself."

## Release Management & Versioning — a genuine, well-verified bridge

This page's "release versioning tracks changes across deployments, ensuring visibility,
traceability" is almost exactly what the `zalmi-tech/developer` repo's real image-tagging
pipeline does, phase 2 of its verified GitOps arc: every app's CI computes a real
per-commit tag (`git rev-parse --short HEAD`) and a reusable action
(`geomachine/bump-image-tag-action@v1`) commits that tag into the infra repo, giving a
literal, auditable Git history of exactly which commit-sha image was live at any point in
time — replacing an earlier state where everything was tagged `:latest` (which gave a
reconciling controller no diff to act on and no way to answer "what's actually deployed
right now"). A real, sharp edge already hit here and worth mentioning as depth, not just
the happy path: re-running CI on an unchanged commit computes the *same* tag, so the
bump-action correctly no-ops (same tag = no diff) and nothing redeploys even though the
image was rebuilt — the fix is pushing a real new commit (even an empty one), not
re-dispatching the same ref. This is real operational knowledge of what "traceable
release versioning" actually requires under the hood, not a surface-level description of
the concept.

## Environment Provisioning (Dev/Staging/Prod) — partial bridge, worth being precise about

The concept — separate, consistent environments for development, staging, and
production — is present in this work, but the *mechanism* differs from what this phrase
usually implies (Terraform modules parameterized per environment). What's actually here:
Kustomize's base/overlay pattern (already verified deep elsewhere) is the real per-
environment mechanism used — a shared `base/` plus environment-specific patches, not
separate Terraform environments. Concretely, hostnames in this repo already encode a
dev/staging distinction (`dev-cdn.skybd365.cloud` currently being planned, a stale
`staging-cdn.skybd365.cloud` already sitting in `backend-1`/`backend-2`'s ConfigMap,
unused, deferred by explicit user choice) rather than fully separate cluster
environments — this is single-cluster, hostname/namespace-scoped separation, not
multi-cluster or multi-account environment isolation. Honest framing: "My environment-
separation experience is Kustomize base/overlay plus hostname-scoped conventions in a
single cluster, not fully isolated dev/staging/prod infrastructure provisioned via
Terraform per environment — if the role means the latter, that's closer to the Terraform
gap already flagged than something I've done."

---

# Cloud Omnium "DevSecOps Implementation" page — term by term

Same method again. Vault, NGINX, and SonarQube are already covered in depth above and in
`interview_prep.md` — not repeated here. ISO 27001/SOC 2 are also already covered above;
this page adds GDPR and PCI-DSS specifically, covered below alongside HIPAA.

## Security-as-Code Integration — the umbrella concept, worth naming plainly

This phrase just means: security policy expressed as version-controlled config that a
pipeline enforces automatically, instead of a manual checklist a person runs by hand —
the same "config, not manual steps" principle already deeply real in this work via
Kustomize/SOPS/ArgoCD, just applied to the security domain specifically rather than
deployment generally. Worth drawing that parallel out loud rather than treating it as an
unfamiliar concept: "The *pattern* — policy as versioned, machine-enforced config — is
the same one my GitOps pipeline already runs on for deployment; I haven't applied that
same pattern to security scanning/policy specifically (SAST/DAST/OPA gates), but the
underlying discipline is one I already practice daily."

## SAST vs. DAST — real gap, keep the distinction crisp

Both are automated vulnerability-finding techniques, but at different points and by
different means — worth being able to state the difference precisely rather than
blurring them, since interviewers listing both back to back are usually checking whether
a candidate actually knows what separates them:

- **SAST (Static Application Security Testing)** — scans *source code* (or bytecode)
  without running it, looking for known-dangerous patterns (SQL built via string
  concat, hardcoded secrets, unsafe deserialization). Runs early, in CI, against every
  commit. Tools: SonarQube, Semgrep, CodeQL.
- **DAST (Dynamic Application Security Testing)** — tests a *running* application from
  the outside, like an attacker would (sends malformed/malicious HTTP requests, checks
  responses for injection/XSS/auth bypass). Needs a live, deployed target — runs later
  in the pipeline, against staging or a test environment. Tools: OWASP ZAP, Burp Suite.

Neither is run anywhere in this repo's pipelines (GitHub Actions build/push/tag steps
only, per the already-established CI gap). Honest framing: "I haven't wired SAST or DAST
into a pipeline myself — I can state precisely what separates them (static source
analysis pre-deploy vs. dynamic black-box testing of a live target), but that's from
knowledge, not hands-on operation."

## Container Image Vulnerability Scanning (Trivy) — a real, partial bridge worth stating precisely

This is the one item on this page with genuine, verifiable evidence in the repo, though
it needs a precise, not overstated, framing. Harbor — already deeply real infrastructure
here (own domain, own values.yaml, own real bugs fixed) — ships **Trivy as its bundled
default image scanner** since Harbor 2.x, and this repo's own
`deployment/base/platform/harbor/values.yaml` provisions real persistent storage for it
(`trivy: storageClass: local-path, size: 5Gi`) alongside the rest of Harbor's component
storage. That confirms the scanner component is actually deployed and has real disk
backing it, not just theoretically available.

**What's *not* confirmed**, and worth being precise about rather than rounding up: nothing
in the values.yaml configures active *enforcement* — no severity threshold, no "prevent
vulnerable images from running" project-level policy, no scan-on-push automation
setting. So the honest claim is: "Harbor's built-in Trivy scanner is deployed in my
registry — I provisioned its storage as part of standing up Harbor — but I haven't
verified or configured active scan-blocking policy on top of it. I'd want to check
Harbor's project settings for whether scanning-on-push and a severity gate are actually
turned on before claiming this is an enforced control today."

## Software Composition Analysis (SCA) / Snyk — real gap, distinct from SAST

SCA specifically means scanning a project's *third-party dependencies* (npm/pip/go
modules, etc.) against known-vulnerability databases (like the NVD or GitHub's own
advisory database) — different target than SAST (which looks at code you wrote) even
though both often get bundled into the same "static scanning" conversation. Snyk is the
most common commercial tool for this, alongside GitHub's own free Dependabot alerts.
Nothing in this repo runs this today. Honest framing: "I haven't wired SCA scanning into
CI — I know the distinction from SAST (dependency vulnerabilities vs. code
vulnerabilities you wrote) but haven't operated Snyk or an equivalent."

## IaC Security Scanning — real gap, but name what it would actually catch here

Tools like Checkov, tfsec, or `kube-score` scan infrastructure config itself (Terraform,
Kubernetes YAML, Dockerfiles) for known-bad patterns *before* it's applied — e.g. a
Kubernetes manifest missing `securityContext`, a container running as root, a Service
accidentally exposed as `LoadBalancer` with no restriction. This repo's manifests already
show real security-conscious patterns by hand (RustFS's non-root `runAsUser: 10001`/
`fsGroup`, Traefik terminating internally behind Caddy at the edge) — but those were
applied by manual review and prior incident-driven learning, not by an automated scanner
enforcing them as a CI gate. Honest framing: "I write security-conscious Kubernetes
config by habit — non-root users, minimal exposed surface — but I haven't run an
automated IaC scanner like Checkov/tfsec to catch this systematically or block a bad
manifest from merging."

## Policy-as-Code (OPA/Gatekeeper) — real gap, and worth distinguishing from what *is* real

Open Policy Agent (OPA) is a general-purpose policy engine (its own query language,
Rego); **Gatekeeper** is OPA packaged as a Kubernetes admission controller — it
intercepts every `kubectl apply`/API write and can reject resources that violate a
policy (e.g. "no image from an untrusted registry," "every Pod must set resource
limits") *before* they're ever persisted to etcd. This is a fundamentally different
enforcement point from anything already real in this repo: ArgoCD's `automated:
{prune, selfHeal}` enforces "does live state match Git" — a drift/consistency policy, not
a content policy. Nothing here validates *what's inside* a manifest before it's applied.
Worth stating that distinction explicitly if it comes up, since it's an easy thing to
conflate: "My real automation enforces that the cluster matches Git — it doesn't enforce
rules about what's allowed to be *in* that Git config in the first place. Gatekeeper-
style admission-time policy enforcement is a gap."

## Runtime Security Monitoring (Falco) — real gap, distinct from the observability already built

Falco watches live kernel syscalls (via eBPF) inside running containers and alerts on
suspicious *behavior* at runtime — a shell spawned unexpectedly inside a container, a
process trying to write to `/etc/shadow`, an outbound connection to an unexpected IP —
security-specific, behavior-based detection. This is a different layer from the
Loki+Promtail stack already deeply real in this repo (`deployment/base/platform/logging`)
— Promtail/Loki does log *aggregation and search* (what did an app print to stdout), not
kernel-level runtime behavioral security monitoring. Worth naming that distinction
plainly rather than implying the existing logging stack covers this: "I have real,
verified log aggregation via Loki/Promtail, but that's observability, not runtime
security monitoring — I haven't run Falco or an equivalent eBPF-based intrusion
detection tool."

## Security Gate Enforcement in CI/CD — same underlying gap as Approval Gates, security-flavored

Mechanically the same concept already flagged above under "Deployment Approval Gates" —
a pipeline stage that blocks progression on failure — just scoped specifically to
security findings (a SAST/DAST/SCA scan failing = pipeline stops) rather than a human
approval click. Same honest framing applies: this repo's pipelines have no gate of any
kind today, security or otherwise — CI builds/tags/pushes, and ArgoCD's fully automated
sync takes it from there with nothing blocking in between.

## WSO2 — likely unfamiliar name, worth knowing what it actually is

WSO2 is an open-source API management and identity platform (API Gateway, Identity
Server for SSO/OAuth2, integration tooling) — a Sri Lanka-founded company, plausibly on
Cloud Omnium's stack list for API gateway/IAM work specifically. No exposure to this
anywhere in observed work; the closest real equivalent here is Traefik (already deep —
ingress routing) plus Headscale (already deep — this repo's own Tailscale-coordination
control plane, itself a form of network identity/access control, just for mesh
networking rather than API/OAuth2 identity). Honest framing if it comes up: "I haven't
used WSO2 specifically — my closest real exposure is Traefik for ingress routing and
Headscale for network-level access control, not API gateway/OAuth2 identity management."

## Compliance Automation — GDPR and PCI-DSS specifically (HIPAA and ISO 27001/SOC 2 covered above)

- **GDPR** — EU data-protection regulation; the technical piece engineers actually touch
  is usually data minimization, encryption at rest/in transit, and a real deletion path
  (a user's data must be genuinely removable on request, not just soft-deleted).
- **PCI-DSS** — payment-card industry standard; the core engineering-relevant rule is
  that raw card numbers should never touch your own systems at all if avoidable (use a
  tokenizing processor like Stripe) — most of PCI-DSS compliance work is architectural
  avoidance, not a control you bolt on after the fact.
- **HIPAA** — US healthcare data-privacy law; similar shape to GDPR technically (access
  controls, audit logging, encryption) but scoped to protected health information
  specifically, with mandatory breach-notification rules.

None of these have been part of any verified work here — SOPS-encrypted secrets and
Zalmi's iGaming-domain context (age-verification/KYC-adjacent, per the platform's own
nature) are the kind of infrastructure that *would* matter for compliance, but no audit
or formal compliance program has been observed. Say plainly if asked: "I haven't worked
under a formal GDPR/PCI-DSS/HIPAA compliance program — I understand what each actually
requires technically, but that's from knowledge, not from having built to satisfy an
auditor."

## Developer Security Training — not a technical gap, just note it plainly if it comes up

This is a process/culture item (secure-coding training for a dev team), not a tool or
mechanism — nothing to bridge or flag technically; if asked, it's simply outside anything
this session's work touched on directly.
