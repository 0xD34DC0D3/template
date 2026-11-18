# appname

The org's Go app template (`appname` is the placeholder name). Spec:
[docs/design.md](docs/design.md). Deploy: [docs/deploy.md](docs/deploy.md).
Starting a new app: [README.md](README.md).

## Build and test

- Needs voodoo checked out beside this repo (`../voodoo`; `go.mod` `replace`).
- `make build` / `make run` (reads `.env`; copy `.env.example`)
- `make test`, `go test -race ./...`, `make vet`, `gofmt -l .` (must be empty)
- `make tools` then `make vulncheck`
- `appname doctor` validates config, checks voodoo's `/healthz` (warns only when `APPNAME_PROD=false`), warns in dev if the Inngest dev server is down, and fails on pending migrations
- `make tools` also installs sqlc and goose; `make sqlc`, `make migrate-new NAME=x`, `make migrate-check`

## Conventions

- Stdlib `net/http` and `http.ServeMux`, not echo. voodoo uses echo, so code
  copied from it needs porting.
- Config is env vars prefixed `APPNAME_`, loaded in `internal/config`; refuse
  unsafe prod settings in `CheckServe` (https `APPNAME_BASE_URL`,
  `APPNAME_VOODOO_URL`, `APPNAME_SITE_URL` in prod).
- Table tests. Comments say why, not what.
- Record decisions and divergences in `docs/design.md`.
- Template changes: keep them generic, and keep the placeholder spelled `appname` / `APPNAME` / `Appname` so `scripts/rename.sh` still finds it.

## Database

- `internal/db.Open` gives `Writer` (one connection, immediate txlock) and `Reader` (read-only pool). Writes and migrations go through `Writer`.
- Schema lives only in `migrations/` (goose, embedded, sequential numbers). Each needs a working `-- +goose Down`; `make migrate-check` proves it.
- Never edit or delete a migration production has applied; undo with a new one.
- Queries go in `internal/db/queries.sql`, ASCII only (see `sqlc.yaml`). Run `make sqlc` and commit the generated code; CI fails if it is stale.
- Times are INTEGER unix seconds.

## HTTP and web

- `internal/server`: routes plus plain `func(http.Handler) http.Handler` middleware (log, headers, recover, 64 KB body cap, CSRF via `CrossOriginProtection`). `server.New(db, auth, jobs, siteURL)` is the whole handler.
- `internal/web`: layout, CSS, fonts, vendored htmx (version and sha256 in `web.go`). `make theme` regenerates `theme.css` from `../website`; do not edit it.
- Render pages with `web.Renderer.Render` and a `web.View{Page, Fragment, ...}`: full page, or the fragment on `HX-Request`. `View.Nav` marks the header link `aria-current` (`NavSub` for a page below it); `View.Wide` gives `main.wide`; `View.Status` sets a non-200 status. Every htmx action must also work as a plain form post (POST, 303 redirect).
- htmx does not swap 4xx: a rejected htmx post answers 200 with the error inside the fragment; a plain post gets the error page.
- Errors are HTML through the layout, never `http.Error`: `s.notFound`, `s.badRequest`, `s.internalError` (`web.Renderer.Error`, which sets `Cache-Control: no-store`). A catch-all `/` pattern gives unknown paths the 404 page (so a wrong method on a known path is 404, not 405; unmatched requests log as route `/`). Panics get the 500 page from `recoverer`.
- The layout's org links use `APPNAME_SITE_URL` (`config.SiteURL`, passed through `server.New` to `web.New`); never hardcode hosts.
- htmx forms set `hx-disabled-elt="find button"`. Style loading with `.htmx-request` (htmx's indicator styles are off).
- CSP forbids inline script and style: no `hx-on`, no `<script>` bodies, no `style=`. htmx is configured by the `htmx-config` meta tag in `layout.html`.
- The request log's route comes from `recordRoute` around the mux, so middleware may call `r.WithContext` freely.

## Jobs

- `internal/jobs`: Inngest Go SDK (`inngestgo`), app ID `appname`. `jobs.New(jobs.Deps{...})` creates the client and registers every function (in `register`); functions get the DB from `Deps`, no globals. Put writes inside `step.Run`.
- The SDK handler is mounted at `/api/inngest` (`jobs.Path`). It skips CSRF and voodoo auth (Inngest signs its requests) and has a 36 MiB body cap instead of 64 KB.
- Config is the SDK's own env vars (`INNGEST_SIGNING_KEY`, `INNGEST_EVENT_KEY`, `INNGEST_DEV`), not `APPNAME_`. Prod requires both keys and refuses `INNGEST_DEV`.
- Local dev: `INNGEST_DEV=1` in `.env`, then `npx inngest-cli@latest dev -u http://localhost:8081/api/inngest`; the UI is at http://localhost:8288.
- Lost events are fine when the work comes due again on its own; no outbox.

## Auth

- voodoo owns sign-in; the app has no users table. `voodooauth.Middleware` (from voodoo's module, `replace`d to `../voodoo` for now) puts the user on the context; read it with `voodooauth.UserFrom(ctx)`. Use voodoo's `id` as the user id.
- Protect routes with `s.auth.RequireUser` / `s.auth.RequireAdmin`. Admin means voodoo's `is_admin`. voodoo down means 503 on protected routes, anonymous elsewhere.
- Local dev: run voodoo on :8080 (`APPNAME_VOODOO_URL`); there is no auth bypass flag.

## Examples

- EXAMPLES, deleted by a new app: `internal/server/example.go`, `views/notes.html`, `views/admin.html`, the nav links in `layout.html`, `migrations/00001_example_notes.sql`, the queries in `queries.sql`, the `appname/example.ping` and hourly notes-count functions in `jobs.go`, and their tests (`TestExampleFunctions`, `TestPingButton`, the notes and nav cases in `server_test.go` and `errors_test.go`).
