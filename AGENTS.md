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

- The migration system has three tracks: `database/migrations/official/` contains upstream migrations and the official install seed (follow upstream updates; never rewrite upstream identities), `database/migrations/local/` contains six Go framework semantic migrations, and `database/migrations/business/` is the framework-user extension track registered with `Register`/`init` and recorded in the independent `business_migrations` ledger. Its contract is documented in `database/migrations/business/README.md`.
- Preserve the migrate order: prefix validation → migration lock → upstream-compatible preflight → install/recovery decision → fresh-snapshot AutoMigrate → three ledger bootstrap/validation steps → official migrations → reconciliation → official seed (fresh/recovery only) → local migrations → business migrations → `local.VerifyCurrent` → current schema validation.
- Migration `Up` functions must be idempotent, prefix-safe, and deduplicate by business keys. Do not use table-empty or `id=1` checks to infer official seed state; seed-owned writes are reliable only after the official seed, which the orchestrator guarantees before local/business `Up` functions run.
- The development database has undergone the approved epoch reset; the local ledger was rebuilt once and renamed to `local_migrations` (formerly `go_migrations`). Official upstream identities remain immutable. Every migration remains prefix-safe (`mysql.prefix` is variable; never hard-code `ba_`). Destructive renames, type changes, and backfills must not rely on AutoMigrate.
- Never hand-edit `cmd/app/wire_gen.go`; after provider or `cmd/app/wire.go` changes run `go generate ./cmd/app`.
- `go run ./cmd/generate` is hazardous: it uses a hard-coded local MySQL DSN and can overwrite generated models relative to the current directory. Inspect it before use.
- `database/migrations/model/*.gen.go` drives the fresh-snapshot AutoMigrate; preserve its tags and migration contracts. `pnpm dev` regenerates `web/types/tableRenderer.d.ts` and i18n Ally language indexes; edit the TypeScript sources under `web/src/lang/` instead. Frontend builds remain in `web/dist/`, while deployment may copy assets into ignored `static/` paths.

## Framework usage best practices (projects built on this framework)

These rules apply when the repository is used as a framework for a business project, not only when developing the framework itself.

- **Table naming: group business tables by category prefix.** Use `<category>_<entity>` so tables, menus, and generated code self-organize: 运营类 `ops_banner`/`ops_support`/`ops_help`, 订单类 `order_recharge`/`order_withdraw`, 用户类 `user_wallet`/`user_level`. Simple names, clear ownership.
- **Three-segment table names: camelCase the tail in CRUD specs.** For a table like `country_language_content`, do not generate `views/.../language_content`; set `webViewsDir: country/languageContent` in the spec (see `crud_specs/country_language_content.yaml`) so the menu name, views directory, and route all stay flat and clean (`country/languageContent`), while the table keeps its readable snake_case name.
- **Commit after every CRUD generation.** Generation touches the model, handler, provider wiring, router, menu rows, and Vue scaffold together; one commit per module makes the change reviewable and keeps `crud:delete`/regenerate round-trips byte-identical. Never mix hand edits into a generation commit.
- **Business schema changes go to the business migration track.** Add one Go file under `database/migrations/business/` calling `business.Register(...)` from `init()` (contract: `database/migrations/business/README.md`). Never add project tables to `official/` or `local/`. Ups must be idempotent, prefix-safe, dedupe by business keys, and must not infer seed state from table emptiness or `id=1`.
- **The permission system is complete — build role hierarchies on `admin` + role groups.** 超级管理员 / 总代理 / 代理 / 员工 style agent systems are implemented with `admin` rows + `admin_group` role assignments + `admin.parent_id` (hierarchy with the `admin_closure` table) — no new auth tables needed. `admin` fields may be fine-tuned (add business columns, drop unused ones); pair every such change with a business-track migration, since destructive column changes must not rely on AutoMigrate.
- **The `user` table is yours to shape.** For frontend-member business you may modify any `user` field, delete unused fields, and adjust the backend member pages (`web/src/views/backend/user`) to match; same migration discipline as `admin`.
- **Frontend portal (`web/src/views/frontend/`) is an example — restyle freely.** Rebuild it into any business-facing portal. Do **not** restyle the admin backend (`web/src/views/backend/`) or alter the admin design system: backend consistency is what lets the project keep merging framework updates cleanly; business admin pages come from the CRUD generator and follow its patterns.

## Installation and test risks

- Web installation creates `conf/config.yaml` and invokes the configured `terminal.commands.migrate.run`; keep that command able to run Cobra `migrate`. The installer is served at `/install` on port 9989.
- MySQL integration tests in `database/migrations/install_test.go` require `BUILDADMIN_TEST_MYSQL_DSN` and mutate schema/data; use only a disposable database. Some legacy tests/generators also assume local MySQL or execute DDL.
- Importing package `tests` creates/loads `conf/config.yaml`. Its `setupRouter()` is currently an empty Gin router because test DI is commented out; login-route tests are not application E2E coverage.
- Air ignores `web/`, tests, and generated Go files and waits 10 seconds before rebuilding. Run Vite separately; if CRUD generation races Air, temporarily increase `.air.toml`’s `build.delay`.
