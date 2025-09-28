# 🕸️ Simple Guide: Setting up a Tailnet

Tailscale lets you connect computers, VMs, and friends across the internet as if they were on the same LAN.

---

## 🔹 Step 1: Create a Tailnet

1. Go to [https://tailscale.com/](https://tailscale.com/)
2. Sign in with **Google, GitHub, or Microsoft**.
3. You now have a **Tailnet** (your private network).

---

## 🔹 Step 2: Install Tailscale

On each machine you want to connect:

* **Linux**:

  ```bash
  curl -fsSL https://tailscale.com/install.sh | sh
  sudo tailscale up
  ```
* **macOS**:
  Download and install from [tailscale.com/download](https://tailscale.com/download).
* **Windows**:
  Download and install `.msi` from the same page.

---

## 🔹 Step 3: Authenticate & Join

Run:

```bash
tailscale up
```

* This opens a browser → log in with your account.
* The device now appears in the **Tailscale Admin Console**:
  [https://login.tailscale.com/admin/machines](https://login.tailscale.com/admin/machines)

---

## 🔹 Step 4: Verify Connection

On any device:

```bash
tailscale status
```

You’ll see all devices + their Tailscale IPs (like `100.x.x.x`).

Ping a machine:

```bash
ping 100.x.x.x
```

---

## 🔹 Step 5: SSH or Access Services

* Standard SSH over Tailscale:

  ```bash
  ssh username@100.x.x.x
  ```
* Enable built-in Tailscale SSH (optional, passwordless & safer):

  ```bash
  tailscale up --ssh
  ```

---

## 🔹 Step 6: Invite Friends

1. Go to [Tailnet Admin Console → Users](https://login.tailscale.com/admin/users).
2. **Invite** your friend’s email.
3. They install Tailscale → run `tailscale up` → log in.
4. Their machine joins your tailnet instantly.

---

✅ Done — now all your machines (and friends’) behave like one private LAN across the internet.
You can now run **Kubernetes clusters, Docker, file sharing, or gaming** as if you were all at home.