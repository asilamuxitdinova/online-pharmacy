#!/usr/bin/env bash
# Runs ON THE SERVER, as the deploy user, called by .github/workflows/deploy.yml:
#
#   remote-deploy.sh <release.tar.gz> <release-name> <health-url>
#
# Unpacks the release next to the previous ones, points `current` at it,
# restarts the service and checks /health. If the new release does not come
# up healthy, `current` is pointed back and the old release restarted.
#
# One-time server setup it relies on (see deploy/README.md):
#   /opt/frege-pharmacy                    owned by the deploy user
#   /usr/local/bin/frege-pharmacy  ->  /opt/frege-pharmacy/current/frege-pharmacy
#   sudoers: deploy user may run `systemctl restart frege-pharmacy.service`
set -euo pipefail

tarball=${1:?usage: remote-deploy.sh <tarball> <release> <health-url>}
release=${2:?release name missing}
health_url=${3:?health url missing}

base=${PHARMACY_BASE:-/opt/frege-pharmacy}
keep=5
service=frege-pharmacy.service

log() { printf '[deploy %s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }

[[ $release =~ ^[A-Za-z0-9._-]+$ ]] || { log "bad release name: $release"; exit 1; }
[ -d "$base" ] || { log "$base does not exist; do the one-time setup in deploy/README.md"; exit 1; }

target="$base/releases/$release"
log "unpacking $release"
rm -rf "$target"
mkdir -p "$target"
tar -xzf "$tarball" -C "$target"
chmod 755 "$target" "$target/frege-pharmacy"
chmod -R a+rX "$target/web"

previous=$(readlink "$base/current" 2>/dev/null || true)

switch_to() {
  ln -sfn "$1" "$base/current.tmp"
  mv -Tf "$base/current.tmp" "$base/current"
}

healthy() {
  for _ in $(seq 1 20); do
    if curl -fsS --max-time 3 "$health_url" | grep -q "\"$1\""; then return 0; fi
    sleep 1
  done
  return 1
}

log "switching current -> $target"
switch_to "$target"
sudo -n systemctl restart "$service"

if healthy "$release"; then
  log "healthy: $release"
else
  log "NOT healthy; recent service log:"
  sudo -n journalctl -u "$service" -n 30 --no-pager 2>/dev/null || true
  if [ -n "$previous" ] && [ -d "$previous" ]; then
    log "rolling back to $previous"
    switch_to "$previous"
    sudo -n systemctl restart "$service"
  fi
  exit 1
fi

# Keep the newest $keep releases. Names start with a UTC timestamp
# (YYYYmmddHHMMSS-sha), so sorting by name is sorting by age.
mapfile -t old < <(find "$base/releases" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort -r | tail -n +$((keep + 1)))
for name in "${old[@]}"; do
  [ "$base/releases/$name" = "$(readlink "$base/current")" ] && continue
  rm -rf "${base:?}/releases/$name"
done
rm -f "$tarball"
log "done"
