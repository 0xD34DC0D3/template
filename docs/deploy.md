# Deploying appname

Same shape as voodoo's deployment: one static binary under systemd behind
Caddy, state in a SQLite file. appname shares voodoo's droplet. Steps that are
identical are not repeated; see [voodoo's deploy doc](../../voodoo/docs/deploy.md)
(`voodoo/docs/deploy.md`) for the Caddy install, firewall, and the S3 bucket,
age key and IAM user.

```
Internet ──▶ Caddy (:443) ──▶ appname (127.0.0.1:8081, systemd) ──▶ /var/lib/appname/appname.db
                                 │  outbound HTTPS: voodoo, Inngest Cloud
Inngest Cloud ──▶ Caddy ──▶ appname /api/inngest   (signed requests)
```

Placeholders: `appname.example.com` (appname's host; `appname.0xd34dc0d3.dev` in
production), `auth.example.com` (voodoo's host), `<server-ip>`,
`root@your-server`, `BUCKET_NAME`.

## First-time server setup

Once, on the droplet that already runs voodoo (Caddy, ufw, `sqlite3`, `age`
and `aws` are already there; if not, follow voodoo's doc).

1. **DNS.** A (and AAAA) record `appname.example.com` to `<server-ip>`, not
   proxied by a CDN, so Caddy can get its certificate. Check
   `dig +short appname.example.com`.

2. **User and data directory:**
   ```sh
   adduser --system --group --no-create-home --home /var/lib/appname appname
   install -d -o appname -g appname -m 0700 /var/lib/appname
   ```

3. **Env file:**
   ```sh
   install -d /etc/appname
   # copy deploy/appname.env.example to /etc/appname/appname.env, set the real values, then:
   chown root:appname /etc/appname/appname.env && chmod 0640 /etc/appname/appname.env
   ```
   Fill in `INNGEST_SIGNING_KEY` and `INNGEST_EVENT_KEY` from step 6. Leave
   `APPNAME_PROD` unset (it defaults to on) and never set `INNGEST_DEV`. See
   [Configuration](#configuration).

4. **systemd unit:**
   ```sh
   # copy deploy/appname.service to /etc/systemd/system/appname.service, then:
   systemctl daemon-reload && systemctl enable appname
   ```
   Later deploys keep the unit in sync.

5. **Caddy.** Add the `appname.example.com` block from `deploy/Caddyfile` to
   `/etc/caddy/Caddyfile` next to voodoo's (never replace the file), then
   back up, `caddy validate`, and `systemctl reload caddy` as the comments in
   that file say.

6. **Inngest Cloud.** In the Inngest dashboard, in a production environment:
   copy the signing key and event key into `/etc/appname/appname.env`, and after the
   first deploy sync the app with URL `https://appname.example.com/api/inngest`
   (Apps, Sync new app). Re-sync after any deploy that adds or changes
   functions, from the dashboard (Resync): the SDK (v0.16+) refuses an
   unsigned `curl -X PUT .../api/inngest` with 401 unless
   `INNGEST_ENABLE_UNAUTHED_SYNC` is set, and appname leaves it unset.

7. **Backups.** Create `/etc/appname/backup.env` (root:root 0600) from
   `deploy/backup.env.example`. Use the same bucket as voodoo, with prefix
   `appname/`, and the same or a new age key. The IAM policy in
   `deploy/aws/backup-iam-policy.json` is scoped to `BUCKET_NAME/appname/*`
   (edit the bucket name); attach it to voodoo's backup IAM user as a second
   statement, or create a second user. Add the same lifecycle rule for the
   `appname/` prefix (`deploy/aws/backup-lifecycle.json`; S3 holds one lifecycle
   configuration per bucket, so merge its rules into voodoo's, do not replace
   them). Cron, in root's crontab, offset from voodoo's:
   ```
   30 4 * * * /usr/local/bin/appname-backup
   ```
   Local-only mode (no off-box variables) works as for voodoo.

8. **First deploy**, from your machine with voodoo checked out beside appname
   (`go.mod` replaces voodoo with `../voodoo`):
   ```sh
   make tools
   APPNAME_HOST=root@your-server make deploy
   ```

9. **Check:** `curl -i https://appname.example.com/healthz` returns 200, and
   `https://appname.example.com/` sends you to `auth.example.com` and back
   signed in. Voodoo must allow appname's host in its cookie domain
   (`VOODOO_COOKIE_DOMAIN=.example.com` covers it).

10. **Test a restore** once, as below.

## Deploying new code

```sh
APPNAME_HOST=root@your-server make deploy
```

`APPNAME_SSH_KEY` picks an identity file. Locally it refuses a dirty tree, runs
`go vet`, `go test` and `deploy/vulncheck.sh`, cross-compiles
`linux/amd64`, and uploads `deploy/install.sh` (as
`/usr/local/sbin/appname-install`) plus the release tarball (`appname`,
`backup.sh`, `appname.service`).

On the server, `appname-install`, in order:

1. installs the backup script and runs it; a failed backup stops the deploy
   with nothing changed;
2. installs the unit and reloads systemd, only if it changed;
3. stops appname (down until step 6);
4. runs `migrate up` with the new binary as user `appname`; on failure restarts
   the old binary and stops;
5. runs `appname doctor` with the new binary and the production env (config,
   voodoo's `/healthz` reachable, migrations applied); on failure restarts the
   old binary and stops. The migration from step 4 stays applied;
6. keeps the old binary as `/usr/local/bin/appname.prev`, installs the new one,
   starts appname, and polls `http://127.0.0.1:8081/healthz` (from `APPNAME_BIND`
   and `APPNAME_PORT`) for about ten seconds. On failure it prints the rollback
   command.

The Caddyfile is never deployed; edit it by hand (step 5 above).

### Rolling back

```sh
install -m 0755 /usr/local/bin/appname.prev /usr/local/bin/appname
systemctl restart appname
curl -fsS http://127.0.0.1:8081/healthz
```

Migrations are not reversed. If the old binary cannot run on the new schema,
restore the backup taken at the start of the deploy (newest file in
`/var/backups/appname`), skipping the `migrate up` step below.

### Deploying from CI

The `deploy` job in `.github/workflows/ci.yml` mirrors voodoo's. Because
`go.mod` replaces voodoo with `../voodoo`, CI checks out voodoo's `main` beside
appname and builds against it (see Known issues in [design.md](design.md)).
CI never uploads `install.sh`; changes to it reach the server only through
`make deploy`. One-time setup, after one `make deploy` so `appname-install` exists:

```sh
ssh-keygen -t ed25519 -N "" -C appname-ci -f ci_key
APPNAME_HOST=root@your-server deploy/ci-setup.sh ci_key.pub
gh secret set DEPLOY_SSH_KEY --env production < ci_key
gh variable set DEPLOY_HOST --body deploy-appname@your-server
gh variable set DEPLOY_KNOWN_HOSTS --env production --body "$(ssh-keyscan -t ed25519 your-server)"
rm ci_key ci_key.pub
```

`DEPLOY_HOST` is a repository variable, not an environment one: the deploy
job's `if` reads it to stay dormant until it is set, and a job-level `if`
cannot see environment variables.

CI connects as `deploy-appname`, not voodoo's `deploy`, so the two keys do not
overwrite each other. Revoke by deleting `/home/deploy-appname/.ssh/authorized_keys`.

## Configuration

Every variable is in `deploy/appname.env.example` (production) and
`.env.example` (local):

- `APPNAME_BASE_URL`, `APPNAME_VOODOO_URL`: https in production; appname refuses to
  start otherwise.
- `APPNAME_SITE_URL`: the org website, linked from the header tile and
  footer (trailing slash trimmed). Defaults to `http://localhost:4321` (the
  website's dev server), so production must set it, to https; appname refuses
  to start otherwise.
- `APPNAME_PORT` (8081) must match the Caddy `reverse_proxy`. `APPNAME_BIND`
  defaults to `127.0.0.1`.
- `INNGEST_SIGNING_KEY`, `INNGEST_EVENT_KEY`: required in production.
  These are the Inngest SDK's own names.
- `APPNAME_PROD`: leave unset.

## Operating

- Logs: `journalctl -u appname -f`; restart: `systemctl restart appname`
- Health: `curl https://appname.example.com/healthz`
- Config check on the server:
  `set -a; . /etc/appname/appname.env; set +a; sudo -u appname --preserve-env="$(sed -nE 's/^([A-Za-z_][A-Za-z0-9_]*)=.*/\1/p' /etc/appname/appname.env | paste -sd, -)" /usr/local/bin/appname doctor`

## Backups

`deploy/backup.sh` (installed as `/usr/local/bin/appname-backup`) is voodoo's
script with appname's names: `sqlite3 .backup` online copy, `integrity_check`,
local gzip in `/var/backups/appname` for 14 days, then age-encrypted upload to
`s3://BUCKET_NAME/appname/`, and an optional healthcheck ping. All
`APPNAME_BACKUP_*` variables mirror voodoo's `VOODOO_BACKUP_*`; see the comments
in `deploy/backup.env.example` and voodoo's doc for the all-or-nothing rule and
local-only mode.

## Restoring from backup

Fetch and decrypt as in voodoo's doc (`aws s3 cp s3://BUCKET_NAME/appname/appname-STAMP.db.gz.age .`,
`age -d -i KEY ... | gunzip > appname.db`), check
`sqlite3 appname.db 'PRAGMA integrity_check;'` prints `ok`, then on the server:

```sh
systemctl stop appname
mv /var/lib/appname/appname.db /var/lib/appname/appname.db.broken
rm -f /var/lib/appname/appname.db-wal /var/lib/appname/appname.db-shm
install -m 0600 -o appname -g appname appname.db /var/lib/appname/appname.db
sudo -u appname env APPNAME_DB_PATH=/var/lib/appname/appname.db /usr/local/bin/appname migrate up
systemctl start appname
```

Anything written after the backup is lost.
