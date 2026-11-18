#!/bin/sh
# Turn a fresh copy of the template into app NAME. Run once, from the repo
# root, right after gonew (which only rewrites the module path):
#
#   sh scripts/rename.sh NAME [PORT]
#
# NAME is lowercase letters and digits (it becomes the binary, cmd/NAME, the
# NAME_ env prefix, the systemd unit and user, /etc/NAME, /var/lib/NAME, the
# deploy-NAME CI user, the backup prefix and the Inngest app ID). PORT, if
# given, replaces the template's 8081 everywhere (pick a free one on a shared
# droplet). The script deletes itself and try-gonew.sh when done.
set -eu

name="${1:-}"
port="${2:-}"
case "$name" in
'' | *[!a-z0-9]* | [0-9]*)
	echo "usage: scripts/rename.sh NAME [PORT]  (NAME: [a-z][a-z0-9]*)" >&2
	exit 2
	;;
esac
case "$port" in
*[!0-9]*)
	echo "PORT must be a number" >&2
	exit 2
	;;
esac
[ -d cmd/appname ] || {
	echo "run from the repo root of an unrenamed template copy" >&2
	exit 1
}

upper="$(printf '%s' "$name" | tr '[:lower:]' '[:upper:]')"
title="$(printf '%s' "$upper" | cut -c1)$(printf '%s' "$name" | cut -c2-)"
self="scripts/rename.sh"

# Every text file except this script and .git. perl -pi behaves the same on
# macOS and Linux, unlike sed -i.
grep -rIl --exclude-dir=.git -e appname -e APPNAME -e Appname . |
	grep -v "^\./$self\$" |
	while IFS= read -r f; do
		N="$name" U="$upper" T="$title" perl -pi -e 's/appname/$ENV{N}/g; s/APPNAME/$ENV{U}/g; s/Appname/$ENV{T}/g' "$f"
	done

if [ -n "$port" ]; then
	grep -rIl --exclude-dir=.git -e 8081 . |
		grep -v "^\./$self\$" |
		while IFS= read -r f; do
			P="$port" perl -pi -e 's/\b8081\b/$ENV{P}/g' "$f"
		done
fi

mv cmd/appname "cmd/$name"
mv deploy/appname.service "deploy/$name.service"
mv deploy/appname.env.example "deploy/$name.env.example"

# Module zips (so gonew) drop file modes.
chmod +x deploy/*.sh scripts/*.sh
rm -f -- "$self" scripts/try-gonew.sh
echo "renamed to $name${port:+ (port $port)}. Left to do by hand: README.md, docs/design.md,"
echo "LICENSE holder, the backup cron time in docs/deploy.md, and the EXAMPLE code."
