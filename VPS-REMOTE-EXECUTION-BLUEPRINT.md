# VPS Remote Execution Blueprint

How to run commands, scripts, installs, and deployments on remote VPS servers —
reverse-engineered from [vitodeploy/vito](https://github.com/vitodeploy/vito)
(a production server-management panel), written **language-agnostic** so it can be
implemented in PHP, Node.js, Python, Go, Rust, or anything else with an SSH library.

Vito is a Laravel app, but nothing in its design depends on PHP. The design is:

> **One SSH wrapper class + shell scripts as templates + typed wrapper modules +
> queued jobs with a per-server lock + streamed logs.**

Every section below describes what Vito does, why, and how to reproduce it in any language.

---

## Table of Contents

1. [Big Picture Architecture](#1-big-picture-architecture)
2. [SSH Key Management](#2-ssh-key-management)
3. [The SSH Wrapper Class](#3-the-ssh-wrapper-class)
4. [Command Execution Semantics](#4-command-execution-semantics)
5. [Error Detection: Exit Codes + Sentinel String](#5-error-detection-exit-codes--sentinel-string)
6. [Writing Files to the Server (the safe way)](#6-writing-files-to-the-server-the-safe-way)
7. [Shell Scripts as Templates](#7-shell-scripts-as-templates)
8. [Typed Wrapper Modules (OS, Systemd, Cron, Services)](#8-typed-wrapper-modules)
9. [Server Provisioning Flow](#9-server-provisioning-flow)
10. [Job Queue + Per-Server Locking](#10-job-queue--per-server-locking)
11. [Live Log Streaming](#11-live-log-streaming)
12. [Running Arbitrary User Scripts Safely](#12-running-arbitrary-user-scripts-safely)
13. [Deployments (release/symlink pattern)](#13-deployments-releasesymlink-pattern)
14. [Security Model](#14-security-model)
15. [SSH Libraries per Language](#15-ssh-libraries-per-language)
16. [Minimal Implementation Checklist](#16-minimal-implementation-checklist)

---

## 1. Big Picture Architecture

```
 ┌───────────────────────────── Control Panel (any language) ─────────────────────────────┐
 │                                                                                        │
 │  HTTP API / UI / Bot command                                                           │
 │        │                                                                               │
 │        ▼                                                                               │
 │  Job Queue (queue name: "ssh")  ──►  Per-server lock ("server-{id}")                   │
 │        │                                                                               │
 │        ▼                                                                               │
 │  Action / Job (InstallServer, DeployJob, ExecuteScriptJob, ...)                        │
 │        │                                                                               │
 │        ▼                                                                               │
 │  Typed wrappers:  server.os()  server.systemd()  server.cron()  service.handler()     │
 │        │              (render shell-script templates with variables)                   │
 │        ▼                                                                               │
 │  SSH wrapper class  ── connect / exec / upload / write / download ──►  phpseclib/ssh2  │
 │        │                                                                               │
 │        ▼                                                                               │
 │  ServerLog: append every output chunk to a log file + broadcast over websocket         │
 └────────────────────────────────────────────────────────────────────────────────────────┘
                                          │ SSH (key auth, port 22)
                                          ▼
 ┌──────────────────────────────── Target VPS (Ubuntu) ───────────────────────────────────┐
 │  user "vito" with passwordless sudo (NOPASSWD:ALL) + panel public key in               │
 │  ~/.ssh/authorized_keys. All commands run through this user; root work via sudo.       │
 └────────────────────────────────────────────────────────────────────────────────────────┘
```

Key decisions:

- **No agent installed on the server for command execution.** Everything is plain SSH.
  (Vito has an optional monitoring agent, but execution is agentless.)
- **The panel never runs the local `ssh` binary for exec.** It uses an SSH *library*
  (phpseclib in PHP) so it can capture output per-chunk, read the exit status, and
  stream output to a callback.
- **All shell logic lives in template files**, not in string concatenation scattered
  through code.
- **All long-running work goes through a queue**, serialized per server with a lock.

---

## 2. SSH Key Management

### Per-server keypair

Every server gets its **own dedicated keypair**, generated when the server record is created:

```bash
ssh-keygen -t ed25519 -m PEM -N '' -f {storage_path}/{server_id}
# produces {server_id} (private) and {server_id}.pub (public)
```

- Stored on the panel's storage disk (filesystem), named by server id.
- The public key is normalized to a single line (newlines stripped) before injection.
- A regeneration command re-derives the public key from the private key:
  `ssh-keygen -y -f {private} > {public}`.

Why per-server keys: revoking one server never affects others; a leaked key
compromises exactly one machine.

### Getting the key onto the server

Two paths:

1. **Cloud providers** (Hetzner, DigitalOcean, Linode, AWS, Vultr...): the panel calls the
   provider API to create the server *with the public key preinstalled* (cloud-init /
   provider ssh-key parameter). Initial login user is `root`.
2. **Custom/existing server**: the user runs a one-line command on their server (or pastes
   the key) that appends the panel's public key to `/root/.ssh/authorized_keys`. For custom
   servers Vito *replaces* the whole authorized_keys with only its unique key
   (`tee` instead of `tee -a`) to guarantee a clean state.

### Connection parameters stored per server

- `ip` (IPv6 supported: wrap in `[...]` when it contains `:`)
- `port` (default 22)
- `ssh_user` (initially `root`, later the created panel user, e.g. `vito`)
- server `status` (installing / ready / disconnected ...)

---

## 3. The SSH Wrapper Class

One class is the single funnel for **all** remote interaction. Its public surface
(reproduce this interface in your language):

```
class SSH:
    init(server, as_user=None)         # bind to a server, load its private key
    connect(sftp=False)                # open SSH2 or SFTP session, key auth
    exec(command, log_name="", site_id=None,
         stream=False, stream_callback=None,
         timeout=0) -> output_string   # run command, capture + log output
    upload(local, remote, owner=None, permission="644")
    download(local, remote)
    write(remote_path, content, owner=None)   # write string content to remote file
    as_user(user)                      # run subsequent commands as another unix user
    variables({KEY: value})            # env vars exported before the command
    set_log(server_log) / use_log(disk, path, callback) / clear_log()
    disconnect()
```

Implementation notes taken from Vito:

- `init()` **resets all state** (connection, log, as_user, variables) and reloads the
  server row from the DB — the wrapper is reused, so stale state is a real bug class.
- The private key is loaded from the per-server key file at init time.
- `connect()` throws a typed `SSHConnectionError`; a failed login throws
  `SSHAuthenticationError`. Command failures throw `SSHCommandError` carrying the log.
- Lazy connection: `exec()` connects on first use if not connected; SFTP operations
  open an SFTP session on demand.
- `disconnect()` is called in the destructor/`finally` so sessions don't leak.
- A **fake/mock twin class** (`SSHFake`) with the same interface is substituted in tests,
  so no test ever needs a real server. `server.ssh()` returns either the real or fake.

---

## 4. Command Execution Semantics

`exec()` never sends the raw command. It builds this envelope:

```
[export VAR1='...'; export VAR2='...'; ]set -e; {command}
```

- Every declared variable becomes `export KEY='escaped value'; ` prefix
  (values escaped with the shell-arg escaper of your language).
- `set -e;` is always prepended → any failing line aborts the script with a
  non-zero exit status.

### Running as a different unix user

When `as_user` is set (e.g. run as an isolated site user instead of `vito`), the command
is wrapped:

```bash
sudo -u {user} bash <<'EOF'
cd ~ || { echo 'VITO_SSH_ERROR: failed to cd to home directory' >&2; exit 1; }
{command}
EOF
```

- Heredoc with **quoted delimiter** (`<<'EOF'`) so the outer shell does not expand
  anything inside — the inner script is passed verbatim.
- `cd ~` first, so relative paths behave like a login shell.
- Works because the connecting user has passwordless sudo.

### Output capture

The SSH library's exec is called with a **per-chunk callback**:

- every chunk is appended to the log (file + websocket broadcast),
- chunks are accumulated into the return string (non-streaming mode),
- in streaming mode the caller's callback also receives each chunk (used for
  interactive-ish views like tailing).

### Timeout

`timeout` parameter (seconds) is set on the SSH channel per call; `0` = unlimited
(used for long installs). Short reads like `systemctl is-active` pass `timeout: 5`.

---

## 5. Error Detection: Exit Codes + Sentinel String

A command **failed** if either:

1. the SSH channel's **exit status ≠ 0**, or
2. the output contains the magic sentinel string **`VITO_SSH_ERROR`**.

The sentinel exists because multi-line scripts can't always rely on exit codes
(pipelines, best-effort sections, sudo layers). Scripts emit it explicitly at known
failure points:

```bash
if ! cd /some/path; then
    echo 'VITO_SSH_ERROR' && exit 1
fi
```

The log writer strips the sentinel out of the text before persisting, so users see clean
logs. On failure, a typed exception is thrown that **carries the log reference**, so the
UI/notification can link straight to the failing output.

Pick your own sentinel (e.g. `BOTGINX_SSH_ERROR`) and apply the same three rules:
scripts emit it on failure paths, the executor treats its presence as failure, the log
writer strips it from display.

---

## 6. Writing Files to the Server (the safe way)

Naive SFTP `put` to `/etc/nginx/nginx.conf` fails: the SSH user isn't root and SFTP has
no sudo. Vito's `write(remote_path, content, owner)`:

1. Write `content` to a **local temp file** on the panel.
2. SFTP-upload it to a random temp name in the SSH user's home
   (`~/{10 random chars}{timestamp}`) — a place the user can always write.
3. `chmod 600 tmp && sudo mv tmp {final_target_or_/tmp}`.
4. Then run (as the target owner) a tiny script:

```bash
if cat '{tmpPath}' > '{path}'; then
    rm -f '{tmpPath}'
else
    rm -f '{tmpPath}'
    exit 1
fi
```

5. `sudo chown owner:owner target` and `sudo chmod {permission} target`.
6. Delete the local temp file in a `finally`.

`upload()` (binary files) is the same pattern without the content-templating step:
SFTP to temp in home → `chmod` → `sudo mv` → `sudo chown` → `sudo chmod`.

`download()` is a plain SFTP `get`.

This gives you: root-owned config writes, correct ownership/permissions, no partial
files at the final path (the `cat >` replaces content atomically enough for configs),
and it works for any target the sudo user can reach.

---

## 7. Shell Scripts as Templates

**The signature pattern.** No shell logic is concatenated in application code. Every
script is a template file, rendered with variables, then passed to `exec()` as one string.

Vito uses Blade (`resources/views/ssh/**/*.blade.php`); in your language use any template
engine (Jinja2, Handlebars, Go text/template, Tera, EJS...). Directory layout mirrors the
domain:

```
templates/ssh/
├── os/                  # create-user, upgrade, install-dependencies, write-file,
│                        # run-script, generate-ssh-key, tail, cleanup, ...
├── services/
│   ├── webserver/nginx/ # install-nginx, nginx.conf template, site vhost, ...
│   ├── php/             # install-php, fpm pool config, ...
│   └── database/        # mysql/postgres install + user/db management
├── git/                 # clone, pull, deploy-key
├── cron/                # update crontab
├── security/            # ufw rules, fail2ban
├── ssl/                 # letsencrypt/custom cert scripts
└── modern-deployment/   # release, link-resources, enable, disable
```

Template example (`create-default-config`):

```jinja
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y nginx
{% if enable_gzip %}
sudo sed -i 's/# gzip on;/gzip on;/' /etc/nginx/nginx.conf
{% endif %}
echo "installed for user {{ user }}"
```

Rules that make this safe and maintainable:

- Interpolated values that come from users go through **shell-arg escaping** in the
  template (`escapeshellarg` equivalent) — paths, names, passwords.
- Non-interactive apt everywhere:
  `sudo DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install -y ...`
- Scripts that need to report a parseable result just `echo "Key: value"` lines and the
  caller parses the output string (e.g. `Packages upgraded: 12`, `Reboot required: 1`).
- Scripts are **idempotent where possible** (`tee` vs `tee -a` chosen deliberately,
  `ln -sfn`, `rm -rf` before re-link).
- Because templates are files in the repo, they are versioned, diffable, and testable
  (Vito snapshots rendered scripts in tests).

---

## 8. Typed Wrapper Modules

Application code never calls `ssh.exec("apt-get ...")` directly. Domain modules wrap the
templates with typed methods. The server object exposes factories:

```
server.ssh(user=None)   -> SSH wrapper
server.os()             -> OS module
server.systemd()        -> Systemd module
server.cron()           -> Cron module
server.security()       -> Security module (ufw/fail2ban)
service.handler()       -> service-specific module (Nginx, PHP, MySQL, Redis, ...)
```

### OS module (the workhorse)

```
os.wait_for_boot(timeout=300)          # loop until cloud-init/system ready
os.install_dependencies()              # base packages: curl zip unzip git gcc openssl ufw cron
os.upgrade() -> {upgraded, reboot_required}
os.create_user(user, password, public_key, clear_keys)
os.create_isolated_user(...)           # per-site unix users
os.run_script(path, script, log, user, variables, aliases)
os.tail(path, lines)                   # read remote logs
os.generate_ssh_key(...) / read public keys / deploy deploy-keys
os.write_file / edit_file / delete_file / cleanup
```

### Systemd module

```
systemd.status(unit)     -> "sudo systemctl status {unit} | cat"
systemd.start/stop/restart/enable/disable(unit)
systemd.active_states([units]) -> "sudo systemctl is-active u1 u2 ... || true"  (5s timeout)
```

Note `| cat` to defeat the pager, and `|| true` + output-line counting for tolerant
status parsing.

### Service handlers

Each installable service (nginx, caddy, php, mysql, postgres, redis, supervisor...) is a
class with a common interface: `install()`, `uninstall()`, `restart()`, plus
service-specific ops. Example — nginx install:

1. `exec(render("nginx/install-nginx"))` — apt install.
2. `ssh.write("/etc/nginx/nginx.conf", render("nginx/nginx.conf", user=ssh_user), owner="root")`.
3. `exec(render("nginx/create-default-ssl"))` — self-signed default cert.
4. Deploy a splash page, then `systemd.restart("nginx")`.
5. Emit `service.installed` event.

This layering means: jobs speak in verbs (`os().upgrade()`), modules speak in templates,
templates speak shell. Each layer is independently testable.

---

## 9. Server Provisioning Flow

`InstallServer` action, run inside a queued job:

```
1. WAIT FOR SSH        loop up to 180s: provider API says "running"? try ssh.connect();
                       on SSHConnectionError sleep 10s and retry.
2. wait_for_boot()     remote script polls until the OS is fully booted (cloud-init done).
3. CREATE PANEL USER   (progress 5%)  — see script below.
4. os.upgrade()        (progress 15%) apt full upgrade, parse "Packages upgraded / Reboot required".
5. install_dependencies() (progress 25%) base packages + git identity + mise (runtime manager).
6. FOR EACH selected service:                    (progress 45% → 100%, split evenly)
       service.new_log(); service.handler().install(); service.status = READY
7. server.status = READY; dispatch follow-up jobs (refresh IPs); send notification.
```

Progress is broadcast to the UI over websockets at each step (`percentage`, `step-name`).
On any SSH error the job's `failed()` hook marks the server/service failed and logs why.

### The create-user script (verbatim logic)

```bash
export DEBIAN_FRONTEND=noninteractive
# custom servers: REPLACE keys; provider servers: append
echo "{public_key}" | sudo tee    /root/.ssh/authorized_keys      # clear_keys=true
echo "{public_key}" | sudo tee -a /root/.ssh/authorized_keys      # clear_keys=false

sudo useradd -p $(openssl passwd -1 {password}) {user}
sudo usermod -aG sudo {user}
echo "{user} ALL=(ALL) NOPASSWD:ALL" | sudo tee -a /etc/sudoers
sudo mkdir /home/{user}
sudo mkdir /home/{user}/.ssh
echo "{public_key}" | sudo tee [-a] /home/{user}/.ssh/authorized_keys
sudo chown -R {user}:{user} /home/{user}
sudo chsh -s /bin/bash {user}
sudo su - {user} -c "ssh-keygen -t rsa -N '' -f ~/.ssh/id_rsa" <<< y   # server's own key (for git)
```

After this, the panel switches `ssh_user` from `root` to the new user and all further
work runs as that user + sudo.

---

## 10. Job Queue + Per-Server Locking

All SSH work is queued (dedicated queue named `ssh`; webhooks etc. use `default`).
The crucial part is the **UniqueQueue** pattern that serializes work per resource:

```
run(key, callback):                       # key = "server-{id}" or "site-{id}"
    lock = cache.lock("unique-queue:" + key, ttl=600s)
    if lock.acquire():
        try:
            callback()
        except TransientDatabaseError e when attempts < tries:
            release_job(delay=min(30, attempts*2))   # retry with backoff
        except e:
            fail(e)                                   # -> job.failed() hook
        finally:
            lock.release()
    else:
        release_job(delay=30s)            # someone else holds the server; requeue

tries = 120; retry_until = now + 1 hour
```

Properties you must reproduce:

- **One operation per server at a time** — two deploys or a deploy+install can never
  interleave on the same machine.
- A job that can't get the lock is **not failed**, it's requeued 30s later, for up to
  an hour.
- Lock TTL (600s) protects against dead workers holding the lock forever.
- Every job has a `failed()` hook that flips the domain object's status
  (deployment FAILED, script execution FAILED...), writes a log entry, and broadcasts
  the update.

Any language stack works: Redis + BullMQ (Node), Celery + Redis lock (Python),
asynq (Go), Sidekiq (Ruby)... The lock is just `SET key NX EX 600` on Redis or an
equivalent cache lock.

---

## 11. Live Log Streaming

Every execution creates a **ServerLog** record:

```
ServerLog {
    server_id, site_id?, name, type, disk, is_remote
    name = "{server_id}-{unix_timestamp}-{type}.log"     # e.g. "42-1724900000-install-nginx.log"
}
```

`write(chunk)` — called from the SSH exec callback for every output chunk:

1. Strip the error sentinel (`VITO_SSH_ERROR`) from display text.
2. Append the chunk to the log file on the panel's storage disk (create if missing).
3. Broadcast a websocket event `{type: "server-log.content", data: {id, content: chunk}}`
   scoped to the project — the browser appends it to the live console view.
   Broadcast failures are caught and swallowed (logging must never break execution).

Two other log modes the wrapper supports:

- `use_log(disk, path, callback)` — write to an explicit file instead of a DB-backed log
  (used by deployments that manage their own log files), optional extra callback.
- **Remote logs** (`is_remote=true`): the "content" is fetched on demand by running
  `tail -n {lines} {path}` on the server — used to view nginx/php logs without copying
  them. A missing file returns sentinel `VITO_NO_FILE` which the UI turns into
  "Log file doesn't exist or is empty!".

---

## 12. Running Arbitrary User Scripts Safely

Feature: users store scripts in the panel and execute them on any server, as any unix
user, with variables. Implementation (`os.run_script`):

```
command  = "set -e\n"
command += "set -o pipefail\n"
command += "shopt -s expand_aliases\n"
for key, alias in aliases:                 # e.g. alias php='php8.3'
    assert key matches ^[A-Za-z_][A-Za-z0-9_]*$   # else refuse to build the command
    command += "alias {key}={shell_escaped(alias)}\n"
for key, value in variables:
    assert key matches ^[A-Za-z_][A-Za-z0-9_]*$
    command += "export {key}={shell_escaped(value)}\n"
command += render("run-script", path=path, script=script)
```

`run-script` template:

```bash
if ! cd {path}; then
    echo 'VITO_SSH_ERROR' && exit 1
fi

{script}
```

Then: `ssh(as_user).exec(command)` inside a queued job with the `server-{id}` lock,
logging to a fresh ServerLog, status transitions RUNNING → COMPLETED / FAILED broadcast
over websockets.

Safety rules encoded here:

- env/alias **names** are validated against a strict identifier regex — never
  interpolated raw;
- env/alias **values** are shell-escaped;
- the script body itself is intentionally raw (it *is* user shell code) but runs as a
  chosen (possibly isolated, non-sudo) unix user;
- `set -e` + `pipefail` so failures propagate;
- everything is logged and attributable.

---

## 13. Deployments (release/symlink pattern)

Vito's "modern deployment" is the classic Capistrano-style layout:

```
/home/{user}/{site}/
├── source/            # persistent git clone (git pull here)
├── releases/
│   ├── 1724900000/    # each deployment copies source -> new release dir
│   └── 1724991111/
├── shared resources   # dirs/files symlinked from source into each release
└── current -> releases/1724991111     # atomic activation
```

Deploy job (on queue `ssh`, locked on `site-{id}`):

1. Update source:
   ```bash
   cd {base}/source
   git stash && git clean -f
   git pull origin {branch}
   ```
2. Create release dir from source; symlink shared resources into it:
   ```bash
   rm -rf {release}/{resource}
   ln -sfn {base}/source/{resource} {release}/{resource}
   ```
3. Run the site's user-defined deploy script inside the release
   (via `run_script`, with variables like commit id available).
4. Activate atomically:
   ```bash
   ln -sfn {releasePath} {base}/current
   ```
5. Post-deploy hooks per site type (restart queue workers, reload php-fpm...),
   mark deployment FINISHED, broadcast, notify.
6. `failed()` hook: mark FAILED and keep/restore the previously active release —
   a failed deploy never takes the site down. Old releases are pruned.

Rollback = re-point `current` at a previous release dir and restart workers.

---

## 14. Security Model

- **Key auth only** for the panel; per-server keypair; private keys on panel disk
  (readable only by the panel).
- Panel user has `NOPASSWD:ALL` sudo — the panel is fully root-equivalent on managed
  servers. The security boundary is the panel itself, not the unix user.
  (Accept this trade-off consciously; it's what makes agentless management practical.)
- **Isolated per-site unix users** (no sudo) for running untrusted site code; the
  `as_user` sudo-wrap runs commands as them.
- Strict identifier validation before emitting any `export`/`alias` statement.
- Shell-arg escaping for every interpolated value in templates.
- Firewall (ufw) and fail2ban configured via the same template mechanism.
- Command allow/deny is enforced at the application layer (policies per panel user);
  the SSH layer executes whatever it's given.
- Sentinel-based failure reporting avoids silently "succeeding" scripts.

---

## 15. SSH Libraries per Language

The whole design needs only these primitives from an SSH library:
**connect with private key → exec with streaming output callback → read exit status →
SFTP put/get → set channel timeout.** Every mainstream language has this:

| Language | Library | Notes |
|----------|---------|-------|
| PHP | `phpseclib/phpseclib` v3 | What Vito uses (`SSH2`, `SFTP`, `PublicKeyLoader`) |
| Node.js / TS | `ssh2` (mscdex) | `client.exec()` gives a stream; `sftp()` built in |
| Python | `paramiko` (sync) / `asyncssh` (async) | asyncssh is ideal for many servers |
| Go | `golang.org/x/crypto/ssh` + `pkg/sftp` | exit status via `*ExitError` |
| Rust | `russh` / `openssh` crate | `openssh` wraps the system client with a control socket |
| Java/Kotlin | `sshj` or Apache MINA SSHD | |
| Ruby | `net-ssh` + `net-sftp` | |
| C# | `SSH.NET` | |
| Elixir | `:ssh` (OTP built-in) | |

Fallback that works everywhere: shell out to `ssh -i key -p port user@host 'bash -s' < script`
and `scp`/`sftp` binaries — you lose clean per-chunk callbacks and exit-status nuance,
but the architecture above still holds.

Queue + lock equivalents: Laravel queue/Cache::lock (Vito) ≈ BullMQ/Redlock (Node) ≈
Celery/redis-lock (Python) ≈ asynq/redsync (Go) ≈ Sidekiq (Ruby).
Websocket broadcast: Laravel Reverb (Vito) ≈ Socket.IO ≈ Django Channels ≈ Phoenix
Channels ≈ any pub/sub → WS bridge.

Template engines: Blade (Vito) ≈ EJS/Handlebars ≈ Jinja2 ≈ Go text/template ≈ Tera.

---

## 16. Minimal Implementation Checklist

Build these, in this order:

1. **Key service**: generate ed25519 keypair per server (`ssh-keygen -t ed25519 -m PEM -N ''`),
   store by server id, expose one-liner for adding the pubkey to a custom server.
2. **SSH wrapper class**: `init/connect/exec/upload/write/download/as_user/variables`,
   lazy connect, per-chunk output callback, timeout per call, typed errors
   (ConnectionError / AuthError / CommandError-with-log), destructor disconnect,
   plus a fake twin for tests.
3. **Execution envelope**: `export`-prefix + `set -e;` + optional
   `sudo -u {user} bash <<'EOF' ... EOF` wrap; failure = exit≠0 OR sentinel in output.
4. **Script templates directory** + render helper; adopt a sentinel string
   (e.g. `BOTGINX_SSH_ERROR`) and the non-interactive apt incantation.
5. **Safe file write**: SFTP to home temp → `sudo mv`/`cat >` → chown/chmod → cleanup.
6. **ServerLog**: `{server_id}-{ts}-{type}.log`, append chunks, strip sentinel,
   broadcast chunk over websocket, remote-tail mode for on-server logs.
7. **Typed modules**: OS (wait_for_boot, upgrade, create_user, run_script, tail),
   Systemd (status/start/stop/restart with `| cat`), then per-service handlers with
   `install/uninstall/restart`.
8. **Queue + lock**: dedicated `ssh` queue, `unique-queue:server-{id}` cache lock
   (TTL 600s), requeue-on-busy 30s, retry window 1h, `failed()` hooks that update
   status + broadcast.
9. **Provision action**: poll-connect (180s) → wait_for_boot → create panel user with
   NOPASSWD sudo → upgrade → base deps → per-service install with progress broadcast.
10. **Run-script feature**: identifier-validated env/aliases, `set -e; set -o pipefail`,
    cd-or-sentinel-fail, as-user execution, live-logged.
11. **Deploy pipeline** (if deploying apps): source clone → release dir → shared
    symlinks → user deploy script → `ln -sfn ... current` → post-hooks → prune;
    failed deploys keep the previous release active.

---

*Source analyzed: vitodeploy/vito @ main, 2026-08-29 (shallow clone). Key files:*
`app/Helpers/SSH.php`, `app/SSH/OS/{OS,Systemd,Cron}.php`, `app/Models/{Server,ServerLog}.php`,
`app/Actions/Server/InstallServer.php`, `app/Jobs/{Site/DeployJob,Script/ExecuteJob}.php`,
`app/Traits/UniqueQueue.php`, `app/Services/Webserver/Nginx.php`, `resources/views/ssh/**`.
