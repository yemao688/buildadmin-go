# Repository Notes

## Project identity and porting principles

- 本仓库是把 PHP BuildAdmin 的生态、接口兼容性和业务语义迁移到 Go 的框架，不是逐行翻译 PHP。
- 需要理解行为时先查上游语义，再结合本仓库实现；不要盲抄 PHP。Go 代码以强类型、Gin/GORM、显式错误处理和仓库既有模式为准。
- 任何兼容性差异都必须配套测试、迁移或文档说明。状态语义按字段区分：`admin.status` 和 `user.status` 的规范值是 `enable/disable`；权限、分组、安全规则和字典等其它状态字段仍按既有协议使用 `0/1`。
- 账户状态迁移由 `database/migrations/local/0001.go` 及其 helper 负责，将历史账户值 `0/1` 转换为 `disable/enable`；API 对账户状态只接受 `enable` 或 `disable`。不要把账户状态规则推广到其它状态字段，也不要把不存在的 `1/2` 转换假设写进新代码。

## Toolchain and boundaries

- Trust `go.mod`: use Go 1.25.x; do not retain the stale Go 1.21.8 requirement.
- This repository contains two projects. The Gin/GORM/Wire backend is rooted here; `web/` is the BuildAdmin v2.3.7 Vue/Vite 8 frontend with its own `pnpm-lock.yaml`. Run frontend commands from `web/` with pnpm, never npm.
- Real entrypoints and wiring are `cmd/app/main.go`, `cmd/app/wire.go`, `router/router.go`, and `web/src/main.ts`. Cobra commands live under `app/cmd/`.
- `conf/config.example.yaml` is the tracked template. Runtime `conf/config.yaml` is ignored and is copied from the template when missing; never commit installer-written credentials.

## AI development protocol

- 先定位现有模式、真实入口和路由边界，再修改；优先最小范围变更，禁止无关重构。
- 业务模块必须使用 CRUD 生成链，不得手写生成的 model、handler、provider 或 Vue 脚手架。先读 `docs/crud-generation.md` 并写 `crud_specs/*.yaml`。
- 数据库、生成器和部署命令先检查副作用。新增依赖或架构变化必须说明理由；不要把未经验证的命令、CI、lint wrapper 或全局检查加入流程。
- 新增用户可见 UI 时同步检查权限、菜单、i18n 以及前后端 API 契约。
- 路由边界：`/admin/*` 是后台路由，`/api/*` 是公共、用户和安装 API。AdminLog 只记录后台 POST/DELETE，不要扩大到所有 API。

## Commands

```bash
# backend, repository root
air                                    # builds ./cmd/app; serves on 9989
go build ./...
go test ./path/to/package -run '^TestName$'
go run ./cmd/app --conf config.yaml migrate
go generate ./cmd/app                  # after provider or cmd/app/wire.go changes

# frontend, web/ (Vite 8; use a current Node release supported by Vite 8)
pnpm install --frozen-lockfile
pnpm dev                               # Vite 9988; API http://localhost:9989
pnpm lint
pnpm typecheck
pnpm build                             # emits web/dist/
```

- Backend changes: run affected package tests and `go build ./...`. Frontend changes: run `pnpm lint`, `pnpm typecheck`, then `pnpm build` from `web/`.
- Do not require default `go test ./...` or `go vet`; choose affected tests because some tests and generators need MySQL or have incomplete application DI. There is no repository CI workflow, task runner, Makefile, or configured Go linter.

## CRUD module generation (AI-driven)

When asked to generate a module, read `docs/crud-generation.md`, create `crud_specs/<module>.yaml`, then run:

```bash
go run ./cmd/app --conf config.yaml crud:generate crud_specs/<module>.yaml [--skip-menu]
go run ./cmd/app --conf config.yaml crud:delete <table_name>
```

Exit code 0 means success and 1 means failure (reason on stderr). Files auto-restore on failure, but MySQL DDL is not rollbackable. Protected core tables are refused.

## Migrations and generated/deployed files

- The migration system has two tracks: upstream-compatible `database/migrations/official/`, Go-local `database/migrations/local/`, and shared internals in `database/migrations/internal/`. Confirm behavior in `app/cmd/handler/migrate.go` and the registries before changing it; do not describe or recreate an old `upgrade.go/allMigrations` layout.
- Official entries retain upstream BuildAdmin identities; local entries have Go-local IDs and may carry legacy aliases. Never rewrite historical identities. Every migration must be idempotent and prefix-safe (`mysql.prefix` is variable; never hard-code `ba_`). Destructive renames, type changes, and backfills must not rely on AutoMigrate.
- Preserve the migrate handler’s order: prefix validation and legacy normalization → install/recovery decision → ledgers and AutoMigrate for the fresh snapshot → official registry → reconciliation/adoption → local registry → schema validation and pending seed.
- Never hand-edit `cmd/app/wire_gen.go`; after provider or `cmd/app/wire.go` changes run `go generate ./cmd/app`.
- `go run ./cmd/generate` is hazardous: it uses a hard-coded local MySQL DSN and can overwrite generated models relative to the current directory. Inspect it before use.
- `database/migrations/model/*.gen.go` drives the fresh-snapshot AutoMigrate; preserve its tags and migration contracts. `pnpm dev` regenerates `web/types/tableRenderer.d.ts` and i18n Ally language indexes; edit the TypeScript sources under `web/src/lang/` instead. Frontend builds remain in `web/dist/`, while deployment may copy assets into ignored `static/` paths.

## Installation and test risks

- Web installation creates `conf/config.yaml` and invokes the configured `terminal.commands.migrate.run`; keep that command able to run Cobra `migrate`. The installer is served at `/install` on port 9989.
- MySQL integration tests in `database/migrations/install_test.go` require `BUILDADMIN_TEST_MYSQL_DSN` and mutate schema/data; use only a disposable database. Some legacy tests/generators also assume local MySQL or execute DDL.
- Importing package `tests` creates/loads `conf/config.yaml`. Its `setupRouter()` is currently an empty Gin router because test DI is commented out; login-route tests are not application E2E coverage.
- Air ignores `web/`, tests, and generated Go files and waits 10 seconds before rebuilding. Run Vite separately; if CRUD generation races Air, temporarily increase `.air.toml`’s `build.delay`.
