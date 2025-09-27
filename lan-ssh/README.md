# 🔐 SSH Setup Between Ubuntu & macOS (Password Protected)

This guide shows how to enable and configure SSH between your **Ubuntu PC (Ethernet)** and **macOS laptop (Wi-Fi)**, both on the same router.

---

## 🚀 On Ubuntu (Linux PC)

### 1. Install & enable SSH server

```bash
sudo apt update
sudo apt install openssh-server -y
sudo systemctl enable ssh
sudo systemctl start ssh
```

### 2. Verify SSH is running

```bash
sudo systemctl status ssh
```

Should show **active (running)**.

### 3. Allow SSH in firewall (if UFW is enabled)

```bash
sudo ufw allow ssh
sudo ufw enable   # only if firewall isn’t already enabled
sudo ufw status
```

### 4. Configure password login

Edit SSH config:

```bash
sudo nano /etc/ssh/sshd_config
```

Ensure:

```
PasswordAuthentication yes
PermitRootLogin no
```

Save & restart SSH:

```bash
sudo systemctl restart ssh
```

### 5. Find username & IP

```bash
whoami   # shows your Ubuntu username
ip a | grep inet   # find your local IP (e.g., 192.168.0.101)
```

✅ Connect from Mac:

```bash
ssh ubuntu_username@192.168.0.101
```

---

## 🍏 On macOS

### 1. Enable SSH server (Remote Login)

```bash
sudo systemsetup -setremotelogin on
```

Or via **System Settings → Sharing → Remote Login** ✅.

### 2. Confirm SSH is running

```bash
sudo systemsetup -getremotelogin
```

Should return:

```
Remote Login: On
```

### 3. Configure password login (optional)

Edit config:

```bash
sudo nano /etc/ssh/sshd_config
```

Ensure:

```
PasswordAuthentication yes
PermitRootLogin no
```

Reload SSH service:

```bash
sudo launchctl stop com.openssh.sshd
sudo launchctl start com.openssh.sshd
```

### 4. Find username & IP

```bash
whoami         # shows your Mac username
ifconfig | grep inet   # look for Wi-Fi IP, e.g., 192.168.0.102
```

✅ Connect from Ubuntu:

```bash
ssh mac_username@192.168.0.102
```

---

## ✅ Verification

* **From Mac → Ubuntu:**

  ```bash
  ssh ubuntu_username@192.168.0.101
  ```
* **From Ubuntu → Mac:**

  ```bash
  ssh mac_username@192.168.0.102
  ```
* First connection will ask:

  ```
  Are you sure you want to continue connecting (yes/no/[fingerprint])?
  ```

  Type `yes` and enter the password.

---

⚠️ Notes:

* Only **normal users** can log in via SSH (not root, by default).
* Both machines must be on the same LAN (same router).
* If connection is refused, check firewall settings and IPs.

---