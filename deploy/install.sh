#!/usr/bin/env bash
# Runs on the server as root. Reads a tar.gz of appname, backup.sh and
# appname.service on stdin; it is the only command the CI deploy key may run.
set -euo pipefail
BIN=/usr/local/bin/appname
ROLLBACK="install -m 0755 $BIN.prev $BIN && systemctl restart appname"

# CI pipes this script's output into a public Actions log, so failure detail
# that can name hosts, addresses or buckets goes to the journal (tag
# appname-install) and the log only says where to look.
journal() { logger -t appname-install -p user.err -- "$1" 2>/dev/null || true; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
tar -xzf - -C "$work" --no-same-owner --no-same-permissions appname backup.sh appname.service

install -m 0755 "$work/backup.sh" /usr/local/bin/appname-backup
install -m 0755 "$work/appname" "$BIN.new"

echo ">> pre-migration backup"
if ! /usr/local/bin/appname-backup; then
	rm -f "$BIN.new"
	echo "!! backup failed; nothing changed, service untouched" >&2
	exit 1
fi

# daemon-reload only when the unit changed (cmp also fails on first install,
# which is what we want).
if ! cmp -s "$work/appname.service" /etc/systemd/system/appname.service; then
	echo ">> unit changed; installing and reloading systemd"
	install -m 0644 "$work/appname.service" /etc/systemd/system/appname.service
	systemctl daemon-reload
fi

# NOT automated: /etc/caddy/Caddyfile. It may hold other sites' blocks, so
# neither copying nor appending is safe. Apply Caddy changes by hand.

set -a; . /etc/appname/appname.env; set +a
# sudo-rs (Ubuntu's sudo) rejects a bare --preserve-env, so name the variables.
env_vars="$(sed -nE 's/^([A-Za-z_][A-Za-z0-9_]*)=.*/\1/p' /etc/appname/appname.env | paste -sd, -)"

# On the first deploy there is no old binary to fall back to.
restart_old() { if [[ -f "$BIN" ]]; then systemctl start appname; fi; }

echo ">> stopping service"
systemctl stop appname

echo ">> migrating with the new binary"
if ! sudo -u appname --preserve-env=APPNAME_DB_PATH "$BIN.new" migrate up; then
	rm -f "$BIN.new"
	restart_old
	echo "!! migration failed; old binary restarted (if any). If the schema is now" >&2
	echo "!! inconsistent, restore the backup above (docs/deploy.md)." >&2
	exit 1
fi

# doctor needs the migrated schema, so it runs after migrate. In prod it also
# requires voodoo's /healthz to answer.
# Its errors can carry voodoo's resolved address, so they go to the journal.
echo ">> doctor (new binary, production config)"
if ! doctor_out="$(sudo -u appname --preserve-env="$env_vars" "$BIN.new" doctor 2>&1)"; then
	journal "doctor failed: $doctor_out"
	rm -f "$BIN.new"
	restart_old
	echo "!! doctor failed; old binary restarted (if any). Details: run" >&2
	echo "!! 'journalctl -t appname-install' on the server. Fix the config or voodoo," >&2
	echo "!! then redeploy. The migration above was already applied." >&2
	exit 1
fi
printf '%s\n' "$doctor_out"

echo ">> installing (previous binary kept as $BIN.prev)"
if [[ -f "$BIN" ]]; then cp -p "$BIN" "$BIN.prev"; fi
mv "$BIN.new" "$BIN"
systemctl start appname

addr="${APPNAME_BIND-127.0.0.1}"
[[ -n "$addr" && "$addr" != "0.0.0.0" && "$addr" != "::" ]] || addr=127.0.0.1
[[ "$addr" == *:* ]] && addr="[$addr]"
url="http://$addr:${APPNAME_PORT:-8081}/healthz"

# appname's /healthz answers 200 with an empty body (voodoo's says "ok"), so the
# status code is the check.
echo ">> health check $url"
for _ in $(seq 10); do
	if curl -fsS -m 3 -o /dev/null "$url" 2>/dev/null; then
		echo ">> healthy"
		echo ">> If this deploy added or changed Inngest functions, re-sync the app"
		echo ">> in the Inngest dashboard (docs/deploy.md); until then they do not run."
		exit 0
	fi
	sleep 1
done
# The service's own log is production data; it stays on the server.
echo "!! health check failed; run 'journalctl -u appname' on the server." >&2
echo "!! Roll back the binary with:" >&2
echo "!!   $ROLLBACK" >&2
echo "!! Migrations are not reversed; see docs/deploy.md for restoring the backup." >&2
exit 1
