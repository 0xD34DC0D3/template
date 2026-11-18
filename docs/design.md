# appname design

The org's Go app template: one binary, SQLite, server-rendered pages, sign-in
by voodoo, jobs on Inngest. Split out of scry (scry's milestone 1, plus later
generic fixes). New apps start from it with `gonew`; see the README.

Replace this file's first paragraph with what the app is for, and add its own
sections (features, data, jobs) below. Keep the rest.

## Stack

- Go, SQLite (`modernc.org/sqlite`), goose, sqlc. Same setup as voodoo.
- HTTP: stdlib `net/http` (`ServeMux` patterns), not echo.
- `html/template` + htmx, voodoo's layout and CSS. Handlers return a full page, or a fragment on `HX-Request`.
- Auth: voodoo session cookie or API key, via `/me`.
- Jobs: Inngest Cloud.

## Shape

- `cmd/appname` (`serve`, `migrate`, `doctor`) and `internal/`.
- Env config, `APPNAME_` prefix; refuse unsafe prod settings. `APPNAME_PROD` defaults to true so a server missing it fails safe; `.env.example` turns it off.
- `APPNAME_SITE_URL` is the org website the header tile and footer link to (the design guide: hosts come from env, never code). Same as voodoo's `VOODOO_SITE_URL`: default `http://localhost:4321`, trailing slash trimmed, https required in prod (`CheckServe`, so prod refuses the default). It reaches the layout through `server.New` and `web.New`, no package global.
- SQLite: WAL, one writer plus a read pool. Goose migrations in `migrations/`, embedded; sqlc reads them as its schema.
  Writer: `MaxOpenConns(1)`, `_txlock=immediate`. Reader: `mode=ro` (modernc supports it). sqlc output lives in `internal/db` beside `Open`; `doctor` fails on pending migrations.
- HTTP: timeouts, body caps, `CrossOriginProtection`, security headers, `/healthz`.
- Errors render through the layout (`web.Renderer.Error`), always `no-store`: the page names the signed-in user. A catch-all `/` pattern gives unknown paths the HTML 404, at the cost of 405s becoming 404s (a `GET /` catch-all would conflict with the method-less `/api/inngest`). Panics get the 500 page; it shows the signed-out header, since `recoverer` runs outside auth. A template error is a bare 500, also `no-store`.
- The request log names the route pattern, never the URL (paths and queries can carry tokens). `recordRoute` around the mux carries the pattern back past middleware that copies the request.
- Header nav: `View.Nav` marks the current link `aria-current="page"`, `"true"` with `NavSub` for a page below it. `View.Wide` gives `main.wide` for table-heavy pages. Under 600px the header is not sticky.
- voodoo auth middleware, `/me` cached 60 s. The only shared Go code: package `voodooauth` in voodoo's module (`github.com/0xD34DC0D3/voodoo/voodooauth`), stdlib only, plain `net/http` middleware. Everything else is copied.
- Admin is voodoo's `is_admin`, via `RequireAdmin`; no local role table or option.
- Web layout, CSS, vendored htmx. CSP bans inline script and style, so htmx runs with `includeIndicatorStyles` and `allowEval` off (meta tag); no `hx-on`.
- Static files are fingerprinted (`?v=` hash): immutable cache when versioned, 1 h otherwise (fonts, loaded from the CSS).
- Inngest client, `/api/inngest`, `jobs/`.
  Inngest env vars keep the SDK's names (`INNGEST_*`), not `APPNAME_`, since the SDK reads them itself; prod requires both keys and refuses `INNGEST_DEV`. `/api/inngest` skips CSRF and voodoo auth (signature-authenticated) and caps bodies at 36 MiB (Inngest's 32 MiB run-state limit plus headroom) instead of 64 KB. Steps return small values (ids, counts): each Inngest call must finish inside `serve`'s 30 s write timeout and arrive inside its 10 s read timeout.
- Pinned tools, CI, govulncheck, `deploy.sh`, `backup.sh`, Caddyfile, systemd.
  Deploy diverges from voodoo's in four ways: `install.sh` runs `appname doctor` (new binary, prod env) after `migrate up` and before installing it; the health check accepts any 2xx because `/healthz` has an empty body; `ci-setup.sh` makes user `deploy-appname` (not `deploy`) so it does not overwrite voodoo's CI key on the shared droplet; the systemd unit leaves outbound HTTPS open (voodoo, Inngest Cloud). The CI deploy job stays dormant until the repository variable `DEPLOY_ENABLED` is `true`. CI checks out voodoo's `main` beside the app for the `../voodoo` replace (see Known issues). `install.sh` reminds the operator to re-sync the Inngest app, which is manual.
- Deploy output is public (Actions logs of a public repo), so it must not show the SSH user or port, sshd's banner, production logs (journal output), the backup bucket's name or the healthcheck URL. The server's IP is public anyway through the app's DNS A record, so masking it is only defence in depth. The SSH host and known_hosts are environment secrets, not variables (a step's env block prints variables in clear; job-level `if` cannot read secrets, hence the separate `DEPLOY_ENABLED` variable), and the deploy step `::add-mask::`s the bare host and each known_hosts entry and host name, which whole-secret masking misses. On the server, `install.sh` sends doctor's failure output (it can carry voodoo's resolved address) to the journal (`logger -t appname-install`) and answers a failed health check with "run journalctl -u appname" instead of `systemctl status`; `backup.sh` runs `age`, `aws` and the healthcheck `curl` through `quiet`, which logs their stderr to the journal (`-t appname-backup`) and prints a generic line. Exit codes are unchanged, and so is what a successful deploy prints. voodoo's deploy does the same.
- `AGENTS.md`, `docs/design.md`, `docs/deploy.md`.

CI also checks: sqlc output is current, `queries.sql` is ASCII-only (see `sqlc.yaml`), migrations run up, down, up.

Out: Antfly, MCP, GraphQL, anything app-specific.

## Examples

One of each, marked EXAMPLE, deleted by a new app: migration `00001_example_notes.sql` and its queries, the notes page (full page plus htmx fragment, plain-form fallback, validation error inside the fragment), the admin page, the `appname/example.ping` event function and the hourly notes-count cron, their nav links in `layout.html`, and their tests. Once production has applied `00001`, drop the table with a new migration instead of deleting the file.

## License

MIT. The copyright line names the authors of the code the template came from (`0x devs`, as in scry's LICENSE), and renaming the template left it alone: MIT asks copies to keep the notice, so a new app keeps that line and adds its own above it.

## Deferred

- GraphQL (gqlgen), until a non-browser client needs it. Resolvers and htmx handlers would wrap the same code.
- TypeScript template, same shape: env names, deploy scripts, docs, look.

## Known issues

- Stdlib `net/http` here; voodoo uses echo. Code copied from voodoo's server needs porting, not pasting.
- Admin is voodoo's `is_admin`, though voodoo's README says `is_admin` is not an app role. Fine while one person runs everything; per-app roles would need a table.
- Copies made from the template drift apart. Port generic fixes back here by hand.
- `go.mod` replaces voodoo with `../voodoo` until voodoo tags a version: builds use whatever voodoo is checked out (CI: voodoo's `main`), so a build is not pinned to a voodoo commit, and a checkout without a sibling `voodoo/` does not build.
- `voodooauth` calls voodoo's `/me` once per request carrying an unknown or garbage credential (only positive answers are cached; no format check or request coalescing). One call per request, not amplification; revisit if voodoo sees abuse.
- `backup.sh` does not read `/etc/appname/appname.env`. If `APPNAME_DB_PATH` there differs from the default, backups print "nothing to back up" and exit 0 without the healthcheck ping (same as voodoo). Keep the default path.
- Inngest sync is manual from the dashboard: inngestgo v0.16+ refuses unsigned `PUT /api/inngest` unless `INNGEST_ENABLE_UNAUTHED_SYNC` is set. Enabling it would also need `ServeOrigin` pinned to `APPNAME_BASE_URL`, or the synced URL comes from the Host header.
- The design guide names voodoo's URL `<APP>_AUTH_URL`; the template (and voodoo's integration docs) use `APPNAME_VOODOO_URL`. Rename or update the guide.
- Error pages from voodooauth's `RequireUser` / `RequireAdmin` (401/403/503) are voodoo's plain responses, not the layout.
- `web.StaticHandler` serves a directory listing at `/static/`, and a missing static file is a text/plain 404 cached `public, max-age=3600`.
- Pages set no Cache-Control unless the handler does. Pages that need auth send no validators, so nothing caches them in practice; a public page must set its own policy and `Vary: Cookie, Authorization` if it names the user.
- Apps sharing a droplet need distinct ports. This app's port was set when the template was renamed; moving it means changing it in the env examples, the Caddyfile and the docs.
