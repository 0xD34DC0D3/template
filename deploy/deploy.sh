#!/usr/bin/env bash
# Gate, build, ship, back up, migrate, install, health-check.
# First-time server setup and rollback are in docs/deploy.md.
#
#   APPNAME_HOST=root@your-server make deploy
#
# APPNAME_HOST is required. APPNAME_SSH_KEY optionally names an identity file;
# without it ssh and scp use your normal SSH config and agent.
#
# The server-side steps (backup, migrate, install, health-check) are in
# deploy/install.sh, which this installs as /usr/local/sbin/appname-install.
set -euo pipefail
cd "$(dirname "$0")/.."
# go.mod replaces voodoo with ../voodoo for now: keep a voodoo checkout beside this one.

HOST="${APPNAME_HOST:?set APPNAME_HOST=user@host}"
SSH_OPTS=()
if [[ -n "${APPNAME_SSH_KEY:-}" ]]; then SSH_OPTS=(-i "$APPNAME_SSH_KEY"); fi

echo ">> gate: clean tree"
if [[ -n "$(git status --porcelain)" ]]; then
	git status --short >&2
	echo "refusing to deploy a dirty tree; commit or stash first" >&2
	exit 1
fi
command -v govulncheck >/dev/null || { echo "govulncheck not found; run 'make tools'" >&2; exit 1; }

echo ">> gate: vet, test, govulncheck"
go vet ./...
go test ./...
./deploy/vulncheck.sh

rev="$(git rev-parse --short HEAD)"
echo ">> building static binary at $rev"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -ldflags="-s -w" -o bin/appname-linux-amd64 ./cmd/appname

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
cp bin/appname-linux-amd64 "$STAGE/appname"
cp deploy/backup.sh deploy/appname.service "$STAGE/"

echo ">> uploading installer and release to $HOST"
scp ${SSH_OPTS[@]+"${SSH_OPTS[@]}"} deploy/install.sh "$HOST:/tmp/appname-install.new"
ssh ${SSH_OPTS[@]+"${SSH_OPTS[@]}"} "$HOST" 'install -m 0755 /tmp/appname-install.new /usr/local/sbin/appname-install && rm /tmp/appname-install.new'
tar -czf - -C "$STAGE" appname backup.sh appname.service |
	ssh ${SSH_OPTS[@]+"${SSH_OPTS[@]}"} "$HOST" sudo /usr/local/sbin/appname-install

echo ">> deployed $rev"
