# Repository Notes

## Toolchain and boundaries

- Trust `go.mod`: use Go 1.25.x; the Go 1.21.8 requirement in `README.md` is stale.
- This repository contains two projects. The Gin/GORM/Wire backend is rooted here; `web/` is the BuildAdmin v2.3.7 Vue/Vite frontend with its own `pnpm-lock.yaml`. Run frontend commands from `web/` with pnpm, not npm.
- Real entrypoints/wiring: `cmd/app/main.go`, `cmd/app/wire.go`, `router/router.go`, and `web/src/main.ts`. Cobra commands live under `app/cmd/`.
- `conf/config.example.yaml` is the tracked template. Runtime `conf/config.yaml` is ignored and copied from the template when missing; never commit installer-written credentials.

## Commands

```bash
# Backend, from repository root
air                                    # backend only; builds ./cmd/app, serves on 9989
go build ./...
go test ./path/to/package -run '^TestName$'
go test ./database/migrations -run '^TestMigrationRegistry$'
go run ./cmd/app --conf config.yaml migrate

# Frontend, from web/ (Vite 8; use a current supported Node release)
pnpm install --frozen-lockfile
pnpm dev                               # Vite 9988; API http://localhost:9989
pnpm lint
pnpm typecheck
pnpm build                             # emits web/dist/
```

- Before finishing backend work, run affected package tests plus `go build ./...`. For frontend work run `pnpm lint`, `pnpm typecheck`, then `pnpm build`.
- There is no repository CI workflow, task runner, Makefile, or configured Go linter; do not invent wrapper commands.

## CRUD module generation (AI-driven)

When asked to "generate a module" (e.g. 用户订单), do NOT hand-write model/handler/vue scaffolding. Drive the built-in chain:

```bash
go run ./cmd/app --conf config.yaml crud:generate crud_specs/<module>.yaml [--skip-menu]
go run ./cmd/app --conf config.yaml crud:delete <table_name>
```

Write the YAML spec into `crud_specs/` first (schema and example: `crud_specs/user_order.yaml`). Full playbook: `docs/crud-generation.md` — read it before generating. Exit code 0 = success, 1 = failure (stderr has the reason); files auto-restore on failure, but MySQL DDL is not rollbackable. Protected core tables (admin, user, rules, logs...) are refused.

## Generated and deployed files

- Never hand-edit `cmd/app/wire_gen.go`. After provider or `cmd/app/wire.go` changes, run `go generate ./cmd/app` (or Wire from `cmd/app/`).
- `go run ./cmd/generate` is hazardous as a routine step: it uses a hard-coded local MySQL DSN and can overwrite generated models relative to the current directory. Inspect it first.
- `database/migrations/model/*.gen.go` drives `AutoMigrate` despite the generated header. Schema upgrades must keep these tags and the version migration in `database/migrations/upgrade.go` consistent; do not casually regenerate them.
- `pnpm dev` first runs `web/src/utils/dev.ts`, regenerating `web/types/tableRenderer.d.ts` and read-only `web/src/lang/*.json`. Edit renderer Vue files and TypeScript language sources instead.
- A frontend build stays in `web/dist/`; installer/terminal deployment moves its `index.html` and `assets/` into ignored `static/` paths.

## Installation and migrations

- Web installation creates `conf/config.yaml`, then invokes the configured `terminal.commands.migrate.run`; keep that command able to run the Cobra `migrate` command.
- Preserve the migration sequence in `app/cmd/handler/migrate.go`: validate prefix → normalize legacy schema → detect fresh DB → `AutoMigrate` → ordered version migrations → legacy reconciliation → pending seed insertion.
- Historical upgrades use upstream BuildAdmin version identities in ordered `allMigrations` (`Version200`…`Version222`) in `database/migrations/upgrade.go`. Add future schema/data changes there; make every `Up` idempotent and do not collapse versions or rewrite old semantics.
- Do not rely on `AutoMigrate` for destructive renames, type changes, or data backfills. Table names are prefix-dependent (`mysql.prefix`, normally `ba_`); use the existing validated/quoted migration helpers, never hard-code `ba_`.
- Current Go status semantics intentionally remain `0/1`; upstream PHP Version222 status conversions to `enable/disable` or `1/2 → 0/1` are deliberately not copied.

## Test caveats

- MySQL integration tests in `database/migrations/install_test.go` require `BUILDADMIN_TEST_MYSQL_DSN` and mutate schema/data. Use only a disposable database.
- Importing package `tests` creates/loads `conf/config.yaml`. Its `setupRouter()` still returns an empty Gin router because test DI is commented out; login-route tests are not application E2E coverage.
- Some legacy tests/generators contain local MySQL assumptions or execute DDL. Inspect database-facing tests before broad `go test ./...` runs.

## Development quirks

- Air ignores `web/`, tests, and generated Go files and waits 10 seconds before rebuilding. Run Vite separately; generated/test edits do not trigger Air.
- CRUD generation can rewrite many Go/Vue files and run Wire. If generation races Air, increase `.air.toml`'s `build.delay` temporarily.
