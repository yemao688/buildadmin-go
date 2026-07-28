# Repository Notes

## 仓库身份自检（每次会话先做）

本文件会被框架源仓库和所有业务 fork 原样继承。开始任何工作前，先判断你在哪一类仓库：

1. 仓库根存在 `PROJECT.md` → **业务仓库**，安装、升级、协作与代码边界规则见 `docs/framework-workflow.md`。
2. `git remote -v` 中 `origin` 指向 `yemao688/buildadmin-go` → **框架源仓库**（业务 fork 的 `origin` 应指向用户自己的 fork），框架维护规则另见 `docs/framework-maintenance.md`。
3. `origin` 指向别处、但有其它 remote（如 `upstream`）指向 `yemao688/buildadmin-go` → **业务仓库**（尚未创建 `PROJECT.md`），按 `docs/framework-workflow.md` 工作，并提醒用户补建 `PROJECT.md`。
4. 以上都不满足（例如 remote 未配置或被改名）→ 向用户确认，不要默认。

## 术语与读者

| 术语 | 含义 |
|---|---|
| 框架源仓库 / 框架上游 | `git@github.com:yemao688/buildadmin-go.git`，发布分支 `v2` |
| 业务仓库 / 下游 | 用户 fork 出的业务项目仓库，主分支通常为 `master` |
| PHP 上游 | BuildAdmin PHP 原版项目，仅框架维护时需要参考 |
| 框架版本 | 根目录 `VERSION_FRAMEWORK`，框架发行 semver |
| 上游版本 | PHP BuildAdmin 兼容基线，事实源为 `web/package.json` 与 `composer.json` |
| 业务版本 | 业务仓库根 `VERSION`，框架不提供该文件 |

全仓库文档禁止裸用"上游"，必须带限定词。本文未标注读者的章节对两类仓库同时生效；标注"仅框架维护者"的内容在业务仓库中不适用。

## Project identity and status semantics

- 本框架把 PHP BuildAdmin 的生态、接口兼容性和业务语义迁移到 Go，不是逐行翻译 PHP：后端 Go（Gin/GORM/Wire），前端基于 BuildAdmin v2.3.8。与 PHP 上游的同步原则仅框架维护者需要，见 `docs/framework-maintenance.md`。
- 状态语义按字段区分：`admin.status` 和 `user.status` 的规范值是 `enable/disable`；权限、分组、安全规则和字典等其它状态字段仍按既有协议使用 `0/1`。
- 账户状态迁移由 `database/migrations/local/0001.go` 及其 helper 负责，将历史账户值 `0/1` 转换为 `disable/enable`；API 对账户状态只接受 `enable` 或 `disable`。不要把账户状态规则推广到其它状态字段，也不要把不存在的 `1/2` 转换假设写进新代码。

## Toolchain and boundaries

- Trust `go.mod`: use Go 1.25.x; do not retain the stale Go 1.21.8 requirement.
- This repository contains two projects. The Gin/GORM/Wire backend is rooted here; `web/` is the BuildAdmin v2.3.8 Vue/Vite 8 frontend with its own `pnpm-lock.yaml`. Run frontend commands from `web/` with pnpm, never npm.
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
- `pnpm lint` uses the flat config `web/eslint.config.mjs`, a 1:1 port of the PHP upstream v2.3.8 `web/.eslintrc.js` ruleset (lenient: most rules off, findings are warn-level). Warnings on pre-existing code (`vue/no-required-prop-with-default`, `no-unused-vars`, `indent`) are upstream-inherited noise — ignore them; do not touch existing source or tighten the config to silence them. Only act on warnings introduced by your own new/changed code.
- Do not require default `go test ./...` or `go vet`; choose affected tests because some tests and generators need MySQL or have incomplete application DI. There is no repository CI workflow, task runner, Makefile, or configured Go linter.

## CRUD module generation (AI-driven)

YAML contract, field/designType rules, relation and time-field JSON contracts: [`docs/crud-generation.md`](docs/crud-generation.md). Read it before writing any spec.

When asked to generate a module, read that doc, create `crud_specs/<module>.yaml`, then run:

```bash
go run ./cmd/app --conf config.yaml crud:generate crud_specs/<module>.yaml [--skip-menu]
go run ./cmd/app --conf config.yaml crud:delete <table_name>
```

Exit code 0 means success and 1 means failure (reason on stderr). Files auto-restore on failure, but MySQL DDL is not rollbackable. Protected core tables are refused.

Every business table should carry `create_time` and `update_time` as `bigint`; the generated CRUD code maintains both and they stay out of the request DTO.

## Migrations and generated/deployed files

- The migration system has three tracks: `database/migrations/official/` contains PHP 上游 migrations and the official install seed (never rewrite their identities), `database/migrations/local/` contains six Go framework semantic migrations (仅框架维护者可改), and `database/migrations/business/` is the business-repository extension track registered with `Register`/`init` and recorded in the independent `business_migrations` ledger. Its contract is documented in `database/migrations/business/README.md`.
- Migration contracts are split by execution lifetime: `VerifyBaseline` runs once after an applied `Up` succeeds, with failed application retried and completed migrations not rerun; it may use an exact baseline predicate. `VerifySchema` and `VerifyUpgradeData` are standing invariants rerun on every `migrate`, so their predicates must remain compatible with valid business changes.
- The business track is the final source of schema shape and may override framework core columns after the framework baseline. Changing an amount column to `decimal` is a domain change and requires corresponding app-model and arithmetic changes; do not change the column alone. After business migrations, local `VerifySchema`/`VerifyUpgradeData`, `local.VerifyCurrent` (cross-table ownership, closure self-rows, security seed identity, and legacy installer-rule rejection), and `official.ValidateCurrentSchema` (current `user_rule` columns and rule enum) remain standing checks.
- Migration `Up` functions must be idempotent, prefix-safe, and deduplicate by business keys. Do not use table-empty or `id=1` checks to infer official seed state; the orchestrator guarantees the official seed runs before local/business `Up` functions.
- 迁移编排顺序、official/local 维护契约和 epoch reset 历史仅框架维护者需要，见 `docs/framework-maintenance.md`；业务仓库只通过 business 轨道扩展迁移。
- Every migration is prefix-safe (`mysql.prefix` is variable; never hard-code `ba_`). Destructive renames, type changes, and backfills must not rely on AutoMigrate.
- Never hand-edit `cmd/app/wire_gen.go`; after provider or `cmd/app/wire.go` changes run `go generate ./cmd/app`.
- `go run ./cmd/generate` is hazardous: it uses a hard-coded local MySQL DSN and can overwrite generated models relative to the current directory. Inspect it before use.
- `database/migrations/model/*.gen.go` drives the fresh-snapshot AutoMigrate; preserve its tags and migration contracts. `pnpm dev` regenerates `web/types/tableRenderer.d.ts` and i18n Ally language indexes; edit the TypeScript sources under `web/src/lang/` instead. Frontend builds remain in `web/dist/`, while deployment may copy assets into ignored `static/` paths.

## Framework usage best practices (business repositories)

These rules apply when the repository is used as a framework for a business project, not only when developing the framework itself.

- **Table naming: group business tables by category prefix.** Use `<category>_<entity>` so tables, menus, and generated code self-organize: 运营类 `ops_banner`/`ops_support`/`ops_help`, 订单类 `order_recharge`/`order_withdraw`, 用户类 `user_wallet`/`user_level`. Simple names, clear ownership.
- **Business table paths: set `generateRelativePath` explicitly in CRUD specs.** Use the `<分类>.<实体驼峰>` form: `seller_money_log` → `seller.moneyLog`, `country_language_content` → `country.languageContent`. The menu name, views directory, route, and model/handler paths all stay flat and clean at two levels (`seller/moneyLog`), instead of letting auto-derivation split longer names into deeper subdirectories; the table keeps its readable snake_case name. `webViewsDir` (as in `crud_specs/country_language_content.yaml`) remains the lower-level single-path override.
- **Commit after every CRUD generation.** Generation touches the model, handler, provider wiring, router, menu rows, and Vue scaffold together; one commit per module makes the change reviewable and keeps `crud:delete`/regenerate round-trips byte-identical. Never mix hand edits into a generation commit.
- **Business schema changes go to the business migration track.** Add one Go file under `database/migrations/business/` calling `business.Register(...)` from `init()` (contract: `database/migrations/business/README.md`). Never add project tables to `official/` or `local/`. Ups must be idempotent, prefix-safe, dedupe by business keys, and must not infer seed state from table emptiness or `id=1`.
- **业务仓库中的 AI 禁止改动框架轨道与框架级文档。** 不向 `official/`、`local/` 添加或修改迁移；不按业务需要改写框架级文档（含本文的维护条款与 `docs/framework-maintenance.md`）。这些文件保持与框架上游一致，业务仓库才能干净地合并框架升级。
- **Set `columnFields` explicitly to control list display.** Omitted `columnFields` puts every field into the admin list page. Keep form-only fields — `password` design types, secrets/tokens, long text such as remarks or large `content` — in `formFields` only, so they never leak into the list. Keep relation-enriched `remoteSelect`/`remoteSelects` FKs in `columnFields`: the raw FK column is auto-hidden while search and the relation display column keep working.
- **The permission system is complete — build role hierarchies on `admin` + role groups.** 超级管理员 / 总代理 / 代理 / 员工 style agent systems are implemented with `admin` rows + `admin_group` role assignments + `admin.parent_id` (hierarchy with the `admin_closure` table) — no new auth tables needed. `admin` fields may be fine-tuned (add business columns, drop unused ones); pair every such change with a business-track migration, since destructive column changes must not rely on AutoMigrate.
- **The `user` table is yours to shape.** For frontend-member business you may modify any `user` field, delete unused fields, and adjust the backend member pages (`web/src/views/backend/user`) to match; same migration discipline as `admin`.
- **Frontend portal (`web/src/views/frontend/`) is an example — restyle freely.** Rebuild it into any business-facing portal. Do **not** restyle the admin backend (`web/src/views/backend/`) or alter the admin design system: backend consistency is what lets the project keep merging framework updates cleanly; business admin pages come from the CRUD generator and follow its patterns.

## Installation and test risks

- Web installation creates `conf/config.yaml` and invokes the configured `terminal.commands.migrate.run`; keep that command able to run Cobra `migrate`. The installer is served at `/install` on port 9989.
- MySQL integration tests in `database/migrations/install_test.go` require `BUILDADMIN_TEST_MYSQL_DSN` and mutate schema/data; use only a disposable database. Some legacy tests/generators also assume local MySQL or execute DDL.
- Importing package `tests` creates/loads `conf/config.yaml`. Its `setupRouter()` is currently an empty Gin router because test DI is commented out; login-route tests are not application E2E coverage.
- Air ignores `web/`, tests, and generated Go files and waits 10 seconds before rebuilding. Run Vite separately; if CRUD generation races Air, temporarily increase `.air.toml`’s `build.delay`.
