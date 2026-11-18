#!/bin/sh
# Make a new app from this checkout the way a real one is made, without
# publishing: serve the working tree as a throwaway module version from a
# file-based GOPROXY, run gonew against it, link the sibling voodoo for the
# ../voodoo replace, and run scripts/rename.sh.
#
#   scripts/try-gonew.sh OUTDIR [NAME] [PORT]
#
# Leaves OUTDIR/NAME (module example.com/NAME), OUTDIR/voodoo (symlink) and
# OUTDIR/proxy. Then: cd OUTDIR/NAME && go test ./... (or the CI steps).
# rename.sh deletes this script from new apps.
set -eu

out="${1:?usage: scripts/try-gonew.sh OUTDIR [NAME] [PORT]}"
name="${2:-newapp}"
port="${3:-}"
root="$(cd "$(dirname "$0")/.." && pwd)"
mod="$(sed -n 's/^module //p' "$root/go.mod")"
# A new version each run: the module cache never refetches a version it has.
ver="v0.0.0-try.$(date +%s)"

[ -d "$root/../voodoo" ] || { echo "need voodoo checked out at $root/../voodoo" >&2; exit 1; }
command -v zip >/dev/null || { echo "need zip" >&2; exit 1; }
mkdir -p "$out"
out="$(cd "$out" && pwd)"
[ ! -e "$out/$name" ] || { echo "$out/$name exists" >&2; exit 1; }

# Module proxy layout: <module>/@v/{list,<ver>.info,<ver>.mod,<ver>.zip},
# with the module path case-escaped (D -> !d). The zip holds the tree under
# the unescaped <module>@<ver>/, minus local-only files.
proxy="$out/proxy"
rm -rf "$proxy"
esc="$(printf '%s' "$mod" | sed 's/\([A-Z]\)/!\1/g' | tr '[:upper:]' '[:lower:]')"
vdir="$proxy/$esc/@v"
mkdir -p "$vdir" "$proxy/src/$mod@$ver"
(cd "$root" && tar -cf - --exclude ./.git --exclude ./bin --exclude ./.env \
	--exclude '*.db' --exclude '*.db-wal' --exclude '*.db-shm' --exclude ./.claude .) |
	(cd "$proxy/src/$mod@$ver" && tar -xf -)
(cd "$proxy/src" && zip -qr "$vdir/$ver.zip" "$mod@$ver")
rm -rf "$proxy/src"
cp "$root/go.mod" "$vdir/$ver.mod"
echo "$ver" >"$vdir/list"
printf '{"Version":"%s","Time":"2026-01-01T00:00:00Z"}\n' "$ver" >"$vdir/$ver.info"

# The local proxy answers for this module only; anything else (gonew
# itself) falls through to the public proxy; no direct, so nothing asks a
# VCS host for this module. GONOSUMDB: the checksum database has never seen
# this module.
(cd "$out" && GOPROXY="file://$proxy,https://proxy.golang.org" GONOSUMDB="$mod" \
	go run golang.org/x/tools/cmd/gonew@latest "$mod@$ver" "example.com/$name" "$out/$name")

[ -e "$out/voodoo" ] || ln -s "$(cd "$root/../voodoo" && pwd)" "$out/voodoo"
(cd "$out/$name" && sh scripts/rename.sh "$name" $port)
echo "made $out/$name"
