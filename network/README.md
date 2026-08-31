# Project Documentation — Headscale (self-hosted Tailscale control plane)

## Overview

This setup runs a **self-hosted Tailscale coordination server** using **Headscale**, fronted by a web admin UI, inside Docker containers on a single host.
It provides a private Tailscale control plane — device registration, WireGuard key coordination, ACL policy, DERP relay selection — without depending on Tailscale Inc's cloud.

It runs via **Docker Compose on its own VM, independent of any Kubernetes cluster in this repo** — deliberately. The tailnet is often *how* you reach a cluster (e.g. for `kubectl`/SSH over the mesh), so its control plane shouldn't have an availability dependency on the cluster it helps you get to.

State is split across two places:
* **Postgres (Aiven, managed)** — users, nodes, ACL policy. Survives host loss on its own.
* **Local disk (`headscale/lib`)** — only the noise-protocol private key (the server's trust identity to already-enrolled clients). Backed up separately by `scripts/backup-headscale-key.sh` — see [Backups](#backups) below.

---

## Components

### 1. **Headscale**

**Container name:** `headscale`
**Image:** `docker.io/headscale/headscale:v0.28.0` — pinned deliberately, not `:latest`.

> A host restart once re-pulled `:latest` and got a newer Headscale with a stricter config schema than what was actually deployed, causing a crash loop (2026-07). The version is now bumped deliberately, with the config re-validated each time — see the comment above the `image:` line in `docker-compose.yaml`.

**Purpose:** Headscale is an open-source implementation of the Tailscale control server. It handles device registration, WireGuard key coordination, node updates, ACL policy, DERP map serving, and the gRPC/REST APIs Tailscale clients talk to.

**Ports:**

* `8081`: REST/gRPC-gateway API
* `50444`: gRPC interface used by Tailscale clients
* `3478/udp`: STUN, for the embedded DERP relay's NAT traversal (relay itself is currently disabled — see [DERP relay](#derp-relay-configuration) below)

**Volume mounts:**

| Host Path                       | Container Path                  | Purpose                                              |
| -------------------------------- | -------------------------------- | ----------------------------------------------------- |
| `./headscale/config.yaml`        | `/etc/headscale/config.yaml`     | Main configuration file (read-only)                   |
| `./headscale/derp-filter.yaml`   | `/etc/headscale/derp-filter.yaml`| Local overrides on top of the public DERP map (read-only) |
| `./headscale/aiven-ca.crt`       | `/etc/headscale/aiven-ca.crt`    | Aiven's CA cert, for verifying the Postgres TLS connection (public, safe to commit — see below) |
| `./headscale/lib`                | `/var/lib/headscale`             | Noise-protocol private key (server identity) — **gitignored, must be backed up separately** |
| `./headscale/run`                | `/var/run/headscale`             | Runtime socket/pidfiles — gitignored, regenerated on start |

**Database:** managed Postgres on Aiven (`database.type: postgres` in `headscale/config.yaml`), not local SQLite.

> A previous local SQLite setup (bind-mounted, no external DB) lost all node/user data after a host restart exposed a broken volume mount — and even once fixed, the target directory had never actually held a persisted `db.sqlite`. Moved to a managed, durable Postgres instance so Headscale's reliability doesn't depend on this VM's local disk. Connection uses `ssl: verify-full` plus `PGSSLROOTCERT` (set in `docker-compose.yaml`, pointing at the mounted CA cert) for full certificate + hostname verification, not just encryption-in-transit.

ACL policy storage is `policy.mode: database` (also in Postgres) rather than a file — this is what lets the ACL/Policy tab in the Zenith dashboard read and write policy through the Headscale API instead of needing a file mounted and kept in sync.

**Secrets:** the Postgres password is **not** in `config.yaml` (a tracked file). It's injected via the `HEADSCALE_DATABASE_POSTGRES_PASS` environment variable in `docker-compose.yaml`, sourced from a **gitignored `.env`** (Viper maps `HEADSCALE_` env vars to config keys automatically — dots become underscores). Copy `.env.example` to `.env` and fill in real values:

```bash
cp .env.example .env
```

**Networks:**
* `headscale-net`: internal bridge network, for the admin UI to reach the API
* `caddy-net`: external network, for the Caddy reverse proxy to reach it

---

### 2. **Headscale Admin** *(currently active web UI)*

**Container name:** `headscale-admin`
**Image:** `goodieshq/headscale-admin:latest`

> Unlike the `headscale` service, this image is **not pinned**. Given the documented incident above about `:latest` causing an unplanned breaking upgrade, pinning this to a known-good tag is worth doing for the same reason.

**Purpose:** web-based management UI for Headscale — users, nodes, routes.

**Ports:** `3001` (host) → `80` (container)

**Access:**
* Direct: `http://localhost:3001`
* Via Caddy: `https://your_dns_or_subdomain`

**Networks:** `headscale-net` (to reach Headscale) and `caddy-net` (for Caddy)

---

### 3. **Headplane** *(config present, not yet wired in)*

`headplane/config.yaml` configures [Headplane](https://github.com/tale/headplane), a newer alternative admin UI — but **there is no `headplane` service in `docker-compose.yaml`**, so it is not currently running. Treat this as staged/evaluation config, not live infrastructure, until a service block is added.

Before wiring it in:
* `cookie_secret` in that file is currently a **literal placeholder value committed to a tracked config file** (`"aVerySecure32CharacterString1234"`). It should move to a real secret injected the same way `HEADSCALE_DATABASE_POSTGRES_PASS` is — via an env var from `.env` — not sit in plaintext in a tracked YAML file.
* `cookie_secure: true` requires HTTPS; the comment above it ("Cookies do not require HTTPS for local testing") contradicts the value actually set — decide which environment this config targets before enabling it.

---

## DERP relay configuration

`headscale/config.yaml`'s `derp` section merges the public Tailscale DERP map (`derp.urls`, auto-refreshed every `update_frequency`) with a local override file, `headscale/derp-filter.yaml`, which *only* removes specific regions — it doesn't replace the public map.

Two regions are currently excluded:

| Region | Name | Why |
| --- | --- | --- |
| `6` | Bengaluru | Root-caused a recurring "SSH connections hang, no correlation with which node" incident (2026-07-31) to nodes whose home-DERP-region election landed on Bengaluru — confirmed unreachable from this tailnet in a stuck node's own logs, while Singapore (a near-coin-flip on latency) was solid every time. Disabling the region stops the bad election outright. |
| `999` | ZenDevz Skynet DERP (embedded relay) | The embedded DERP relay (`derp.server.enabled`) was tried and reverted the same day: DERP's HTTP/1.1-only upgrade handshake broke intermittently behind Caddy, which serves this same domain over both HTTP/1.1 and HTTP/2. Region 999 is still *advertised* in the map regardless (`automatically_add_embedded_derp_region: true`), so it's excluded explicitly here too, rather than relying on that flag to behave once the server itself is off. |

Re-enabling either requires deleting the corresponding `<id>: null` line in `derp-filter.yaml`. Re-enabling the embedded relay specifically also requires giving Caddy a dedicated HTTP/1.1-only listener first — see the full comment in `headscale/config.yaml`'s `derp` section, and the wider org repo's `TAILNET_INCIDENT_REPORT_2026-07-31.md` for the complete investigation.

---

## Backups

`scripts/backup-headscale-key.sh` backs up `${WORK_DIR}/headscale/lib` (the noise-protocol private key) to the same S3 bucket used by the Postgres backup pipeline. This is the **one piece of Headscale state that lives only on local disk** — everything else (users, nodes, ACL policy) is in the Aiven Postgres instance and covered by its own backups.

Losing this key doesn't lose data, but every already-enrolled client would have to re-register against a new server identity — restoring it avoids that if the host is rebuilt (not truly lost).

```bash
# from network/, with WORK_DIR and AWS_* sourced (normally from .env)
./scripts/backup-headscale-key.sh
```

Scheduled via crontab — see `../BACKUP.md` in the wider org infra repo.

---

## Docker Compose Flow

1. `docker compose up -d`:
   * Creates the `headscale-net` bridge network.
   * Joins the pre-existing external `caddy-net` network.
   * Starts `headscale` and `headscale-admin`.
2. `headscale/config.yaml` tells Headscale where its Postgres database is, what domain it serves clients on, and which DERP regions to filter.
3. After startup:
   * `headscale` listens on `8081` (API/REST), `50444` (gRPC), `3478/udp` (STUN).
   * `headscale-admin` listens on `3001` and talks to Headscale over `headscale-net`.
   * Caddy reverse-proxies `https://your_dns_or_subdomain` to `headscale-admin`.

---

## Management Commands

All Headscale commands are run via `docker exec` on the host running this compose stack.

### Create an API key

```bash
docker exec headscale headscale apikeys create
```

### List users

```bash
docker exec headscale headscale users list
```

### Create a user

```bash
docker exec headscale headscale users create <username>
```

### List nodes

```bash
docker exec headscale headscale nodes list
```

### Generate pre-auth key

```bash
docker exec headscale headscale preauthkeys create --user <username> --expiration 24h
```

### Delete a node

```bash
docker exec headscale headscale nodes delete --identifier <node-id>
```

---

## Example Workflow

1. Start the system:

   ```bash
   cd network
   docker compose up -d
   ```

2. Create a user:

   ```bash
   docker exec headscale headscale users create kraken
   ```

3. Generate a pre-auth key:

   ```bash
   docker exec headscale headscale preauthkeys create --user kraken --expiration 24h
   ```

4. Join a device:

   ```bash
   tailscale up --login-server=https://your_dns_or_subdomain --authkey=<preauth-key>
   ```

5. Manage the tailnet at `https://your_dns_or_subdomain`.

---

## Network Summary

| Component          | Purpose                             | Ports        | Access                          |
| ------------------- | ------------------------------------ | ------------ | -------------------------------- |
| `headscale`         | Control server (Tailscale backend)   | 8081, 50444, 3478/udp | API: `http://localhost:8081` |
| `headscale-admin`   | Active web UI                        | 3001 → 80    | `https://your_dns_or_subdomain`   |
| `headplane`         | Alternative web UI — config only, not yet a running service | — | — |
| Aiven Postgres      | Users, nodes, ACL policy (external, managed) | 19821 (outbound only) | Not reachable via Caddy |
| `headscale-net`     | Bridge network                       | Internal     | Container-to-container          |
| `caddy-net`         | External network                     | —            | Caddy reverse proxy             |

---

## Troubleshooting

### Check container status

```bash
docker ps | grep headscale
```

### View logs

```bash
docker logs headscale
docker logs headscale-admin
```

### Restart services

```bash
cd network
docker compose restart
```

### Verify network connectivity

```bash
docker exec headscale ping headscale-admin
```

### Check Caddy routing

```bash
curl -I https://your_dns_or_subdomain
```

### Common Issues

**Issue:** `headscale-admin` can't reach `headscale`
**Solution:** confirm both containers share `headscale-net`:
```bash
docker network inspect headscale-net
```

**Issue:** Can't reach the web UI via Caddy
**Solution:** confirm `caddy-net` exists and Caddy's own config routes the domain:
```bash
docker network ls | grep caddy
```

**Issue:** Headscale fails to start after a restart, or comes up on an unexpected version
**Solution:** check the pinned tag in `docker-compose.yaml` wasn't changed, and that nothing repulled `:latest` — see the incident note there.

**Issue:** clients report failing/hanging DERP relay connections
**Solution:** check which region the affected node elected as home (`tailscale netcheck` / its `tailscaled` logs); if it's a region not in `headscale/derp-filter.yaml`'s exclusion list but still unreachable from this tailnet, it likely needs adding — see [DERP relay configuration](#derp-relay-configuration).

**Issue:** `/api/v1/policy` calls 500 in the admin UI
**Solution:** confirm `policy.mode: database` in `headscale/config.yaml` — file mode with no path configured is the old failure mode this was moved away from.

---
