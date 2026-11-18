# appname

The 0xD34DC0D3 Go app template: one binary, SQLite, server-rendered pages
with htmx, sign-in delegated to [voodoo](https://github.com/0xD34DC0D3/voodoo),
jobs on Inngest, deploy scripts for a shared droplet. It runs as is, with one
example of each piece (migration, query, page, admin page, job, cron).

Stack and conventions: [docs/design.md](docs/design.md),
[AGENTS.md](AGENTS.md). Deploy: [docs/deploy.md](docs/deploy.md).

## Starting a new app

Delete this section once done.

voodoo is not tagged yet, so `go.mod` has
`replace github.com/0xD34DC0D3/voodoo => ../voodoo`: keep a voodoo checkout
beside the new app (CI checks one out the same way).

```sh
cd ~/src/0xD34DC0D3            # the directory that holds voodoo/
go run golang.org/x/tools/cmd/gonew@latest github.com/0xD34DC0D3/template github.com/0xD34DC0D3/newapp
cd newapp
sh scripts/rename.sh newapp 8082  # name, and a port no other app on the droplet uses
git init && git add -A && git commit -m "feat: newapp from the template"
```

`gonew` rewrites only the module path, and drops file modes (rename.sh
restores `+x` on the scripts). `scripts/rename.sh NAME [PORT]` does the
rest in one pass, then deletes itself: it replaces the placeholder
`appname` / `APPNAME` / `Appname` everywhere, which renames the binary and
`cmd/` dir, the `APPNAME_` env prefix, the systemd unit, service user and
paths (`/etc/appname`, `/var/lib/appname`), the `deploy-appname` CI user, the
S3 backup prefix, the Inngest app ID and event prefix, and the CI checkout
path; with PORT it also replaces the default port 8081.

Then, by hand:

- README.md, the top of docs/design.md, and your copyright line in LICENSE
  (add it; keep the template's, which MIT asks copies to retain).
- The backup cron time in docs/deploy.md (offset it from the other apps').
- Delete the EXAMPLE code (list in AGENTS.md) as the real code replaces it.
  Before the first production deploy, delete `migrations/00001_example_notes.sql`
  and its queries outright; after it, drop the table in a new migration.

To try this from a local checkout without publishing, use
`scripts/try-gonew.sh` (below).

## Run

```sh
cp .env.example .env
make run
curl localhost:8081/healthz
```

`appname doctor` checks the configuration without starting the server.
Signing in needs voodoo running on :8080; jobs need the Inngest dev server
(AGENTS.md).

## Testing the template

`scripts/try-gonew.sh OUTDIR [NAME] [PORT]` makes a new app from this
working tree exactly as above, without publishing: it serves the tree from a
file-based `GOPROXY`, runs gonew (module `example.com/NAME`), links
`OUTDIR/voodoo` to the sibling voodoo, and runs `rename.sh`. Then run the CI
steps (`.github/workflows/ci.yml`) in `OUTDIR/NAME`.

## Theme

`internal/web/static/theme.css` is generated from the org website's design
tokens and committed. `make theme` (`scripts/sync-theme.sh`) regenerates it
and needs the [website](https://github.com/0xD34DC0D3/website) repo checked
out beside this one (`../website`, or `make theme WEBSITE=path`); without it, treat
theme.css as an ordinary committed file.

## License

[MIT](LICENSE), except these third-party files:

- `internal/web/static/htmx.min.js`: [htmx](https://htmx.org), 0BSD; the
  version and sha256 are in `internal/web/web.go`.
- `internal/web/static/fonts/*.woff2`: Atkinson Hyperlegible Next and
  JetBrains Mono, SIL Open Font License 1.1
  (`internal/web/static/fonts/OFL.txt`).
