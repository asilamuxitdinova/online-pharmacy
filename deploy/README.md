# Deployment

`pharmacy.sukoon.uz` runs on the server that already serves `sukoon.uz` behind Caddy.

```
browser / Frege ──https──▶ Caddy ─┬─ /api/*            ─▶ 127.0.0.1:8090 (prefix stripped)
                                  ├─ /oauth/authorize,
                                  │  /oauth/token,
                                  │  /docs, /openapi.yaml ─▶ 127.0.0.1:8090
                                  └─ everything else     ─▶ /opt/frege-pharmacy/current/web
```

## What happens on a deploy

| Branch | What runs |
|---|---|
| `dev` (push or PR) | **CI** only: Go lint and race tests, frontend typecheck and build, shellcheck and the systemd unit check. |
| `main` (push, or *Run workflow*) | **Deploy**: CI, then build the release, then upload it over SSH and run [`remote-deploy.sh`](remote-deploy.sh), then a smoke test through `https://pharmacy.sukoon.uz`. |

A release is one tarball: the static `frege-pharmacy` binary, `web/` (the frontend build) and `VERSION`. On the server it unpacks to `/opt/frege-pharmacy/releases/<release>/`. Then `current` is switched to it, the service restarts, and `/health` must report the new version within 20 seconds. If it doesn't, `current` switches back and the previous release restarts. The last 5 releases are kept.

The binary migrates the database on start (`ExecStartPre`). Data lives in `/var/lib/frege-pharmacy/` and survives deploys. A timer resets it to the demo seed every night at 03:00.

## GitHub settings

Create an environment named **production** (Settings → Environments). Optionally restrict it to the `main` branch and require a reviewer.

| Kind | Name | Value |
|---|---|---|
| secret | `DEPLOY_HOST` | the server's address |
| secret | `DEPLOY_USER` | the deploy user (e.g. `deploy`) |
| secret | `DEPLOY_SSH_KEY` | private key of a key pair made only for this |
| secret | `DEPLOY_KNOWN_HOSTS` | output of `ssh-keyscan -p <port> <host>`, checked against the server |
| variable | `DEPLOY_PORT` | SSH port, if not 22 |
| variable | `HEALTH_URL` | if `ADDR` is not `127.0.0.1:8090`: `http://127.0.0.1:<port>/health` |

## One-time server setup

Done by hand, once, before the first deploy. The full steps come when we configure the server. In outline:

1. A `deploy` user that logs in with the deploy key only, and owns `/opt/frege-pharmacy`.
2. The service user and units: `frege-pharmacy` system user, `backend/deploy/systemd/*` in `/etc/systemd/system/`, and `backend/deploy/pharmacy.env.example` as `/etc/frege-pharmacy/pharmacy.env` with real secrets and `PUBLIC_URL=https://pharmacy.sukoon.uz/api`.
3. `/usr/local/bin/frege-pharmacy` as a symlink to `/opt/frege-pharmacy/current/frege-pharmacy`.
4. A sudoers file letting `deploy` run exactly two commands:
   ```
   deploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart frege-pharmacy.service, /usr/bin/journalctl -u frege-pharmacy.service -n 30 --no-pager
   ```
5. The [`Caddyfile.pharmacy`](Caddyfile.pharmacy) site added to Caddy, then `caddy validate` and a reload.
