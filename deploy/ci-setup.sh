#!/usr/bin/env bash
# One-time: lets CI deploy as user "deploy-appname", whose key can only run appname-install.
#
#   APPNAME_HOST=root@your-server deploy/ci-setup.sh path/to/ci_key.pub
# A separate user from voodoo's "deploy" on the same droplet: both write their
# own authorized_keys, so sharing one account would overwrite the other's key.
set -euo pipefail

HOST="${APPNAME_HOST:?set APPNAME_HOST=user@host}"
SSH_OPTS=()
# IdentitiesOnly: otherwise ssh offers agent keys first and the server can
# refuse with "too many authentication failures" before reaching this one.
if [[ -n "${APPNAME_SSH_KEY:-}" ]]; then SSH_OPTS=(-i "$APPNAME_SSH_KEY" -o IdentitiesOnly=yes); fi
PUB="$(cat "${1:?usage: ci-setup.sh path/to/ci_key.pub}")"

ssh ${SSH_OPTS[@]+"${SSH_OPTS[@]}"} "$HOST" 'bash -s' <<REMOTE
set -euo pipefail
id deploy-appname >/dev/null 2>&1 || useradd --system --create-home --shell /bin/sh deploy-appname
install -d -m 0755 -o root -g root /home/deploy-appname/.ssh
echo 'restrict,command="sudo -n /usr/local/sbin/appname-install" $PUB' > /home/deploy-appname/.ssh/authorized_keys
chmod 0644 /home/deploy-appname/.ssh/authorized_keys
echo 'deploy-appname ALL=(root) NOPASSWD: /usr/local/sbin/appname-install' > /etc/sudoers.d/appname-deploy
chmod 0440 /etc/sudoers.d/appname-deploy
visudo -cf /etc/sudoers.d/appname-deploy
REMOTE
