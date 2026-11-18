# Contributing

Issues and pull requests are welcome. For anything larger than a small fix,
open an issue first so we can agree on the approach; appname deliberately stays
small.

## Setup

- Go version from `go.mod`.
- `make tools` installs the pinned `sqlc`, `goose` and `govulncheck`.
- `make help` lists every target.

## Before sending a change

```sh
gofmt -l .        # must print nothing
make vet
make test         # CI also runs go test -race ./...
make vulncheck
make sqlc         # after editing migrations/ or queries.sql; commit the output
make migrate-check # migrations must run up, down to zero, up
```

## Style

- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/)
  (`feat: ...`, `fix: ...`, `docs: ...`), short and imperative.
- Standard library first; add a dependency only when it earns its keep.
- Update `docs/design.md` when you make a decision or diverge from it.

By contributing you agree that your contributions are licensed under the
project's [MIT license](LICENSE).

## Database changes

- `make migrate-new NAME=add_thing`, then write both `-- +goose Up` and `-- +goose Down`.
- Add queries to `internal/db/queries.sql` (ASCII only), then `make sqlc`.
