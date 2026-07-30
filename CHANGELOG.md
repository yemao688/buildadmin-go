# Changelog

## Unreleased

- 修复 migrations 包 4 个腐化的 MySQL E2E 测试（install/recovery/upgrade 家族）：`TestInstall`、`TestFreshSeedPendingRetryAfterOverlayFailure`、`TestUpstreamSecurityBaselineThenLocalOverlay`、`TestInstallRecoveryDecisionFourStates` 子测试。
- **Breaking (RBAC fail-closed):** 未登记 `admin_rule` 且未声明豁免的 `/admin/*` 路由现在返回 403（此前为告警放行），对齐 PHP 上游语义；超管 `*` 绕过不受影响。豁免通过 `middleware.RegisterPermissionExempt` 在路由注册器声明（ajax `*`、alioss callback、index index/logout、crud 辅助端点、crud/log index（另按上游语义在 handler 内手动检查 `crud/crud/index` 权限）、module state/dependentinstallcomplete）。
- **Fixed:** `uploadCompleted` 移植修复——从 `/admin/module/uploadCompleted`（空壳 stub，前端调用一直 404）移回 `/admin/crud.Crud/uploadCompleted`，并实现 PHP 的 `crud_log.sync` 条件更新语义。
- **Added:** debug 模式启动时输出“未登记 admin_rule 也未声明豁免”的后台路由告警清单。
- **Breaking (运行时目录布局):** 配置、静态资源和运行时目录已切换到新的根目录布局。配置加载无向后兼容回退，这是硬切换：不会再回退读取旧的 `conf/config.yaml`。
- **迁移配置：** 执行 `git mv conf/config.yaml config.yaml`（或手动移动）；配置模板现为根目录的 `config.example.yaml`。
- **迁移静态资源：** 执行 `git mv static public`。所有 HTTP URL 前缀保持不变，`/static/*` 现由 `public/` 提供服务。框架自带的 `fonts/`、`images/` 现位于 `public/static/` 下（与 URL 结构镜像）。
- 删除 `database/buildadmin.sql` 及其升级合约测试（该 SQL 仅为测试夹具；生产安装使用 AutoMigrate + Go 种子，不受影响）。
- **迁移运行时文件：** 执行 `git mv storage runtime`，并将已有配置中的 `log.root_dir` 改为 `runtime/logs`。上传文件现位于 `public/storage/`；如需保留历史上传，将 `storage/default` 等内容移入 `public/storage/`。
- **迁移部署配置：** Docker 入口改为 `--conf /app/config.yaml`，Compose 挂载改为 `./config.yaml` 与 `./runtime`。

## v2.0.2

- **Security:** API tokens are now bound to their account domain. Admin endpoints require an `admin`-type token and user endpoints a `user`-type token; a same-ID cross-domain token no longer authenticates (previously a frontend `user` token could act as an admin with the same ID). Token refresh is bound likewise (`admin-refresh` → `admin`, `user-refresh` → `user`).
- **Security:** every authenticated request now re-checks that the account exists and is `enable`; disabling an admin or user invalidates their sessions immediately instead of waiting for token expiry.
- **Security:** the shared QueryBuilder no longer passes the `order` parameter through as raw SQL. Sorting must match `field,asc|desc` with a strict identifier whitelist, and search field names are validated the same way; malformed values now return 400. (Behavior change: previously-accepted malformed `order` strings are rejected.)
- **Breaking (deploy):** `migrate` no longer applies `crud_specs/*.yaml` automatically by default. Set `crud.apply_on_migrate: true` to re-enable the v2.0.1 deploy loop. The explicit `crud:apply` command is unaffected.
- Fixed `migrate` / `crud:*` commands silently succeeding on failure: migration and apply errors now propagate to a non-zero process exit code.
- Fixed the country dictionary specs drifting from the framework migration baseline: they now declare `unsigned`, defaults and comments matching local migration 0005, and `country_language_content.type` is aligned to `varchar(30)` across spec, snapshot model, admin model, DTO and the frontend (including its `0=文本,1=富文本,2=图片` enum semantics and select widget). Applying the shipped specs after a fresh migrate is now a zero-diff no-op.
- Fixed the admin token refresh branch using `UserTokenKeepTime`; it now uses `AdminTokenKeepTime`.
- Removed the dead legacy `tests/` package (an always-failing login test against an empty router; no real coverage).
- The route snapshot golden file moved to `router/testdata/registered_routes.golden` with a header explaining its purpose and regeneration command.

### Upgrade

1. If your deployment relies on `migrate` auto-applying specs, set `crud.apply_on_migrate: true` in `conf/config.yaml`.
2. Clients sending non-standard `order` values on list endpoints will now receive 400; use `field,asc|desc` with plain column identifiers.
3. No action is needed for correctly issued tokens; only cross-domain token usage (which was a vulnerability) is rejected.

## v2.0.1

- **Breaking:** `app.env` now accepts only `debug` or `release` and drives `gin.SetMode`; the legacy `local` value is a startup config error. Recovery middleware now reads `gin.Mode()`, so `release` mode actually hides internal error details.
- **Breaking (country dictionary modules):** the country modules were regenerated with the `generateRelativePath` subdirectory layout. Go code moved from the flat `app/admin/handler` / `app/admin/model` packages into `app/admin/handler/country` / `app/admin/model/country` (e.g. `handler.CountryCurrencyHandler` → `country.CurrencyHandler`), and their HTTP routes changed from `/admin/countryCurrency/*` (and `countryLanguage*`) to the framework-wide dotted form `/admin/country.Currency/*`, aligning with the seeded `admin_rule` button names (`country/currency/...`). The framework's own admin pages and menus are already migrated; database menu rows seeded by local migration 0005 are unchanged.
- Added the `crud:apply` deployment command: CRUD specs become the source of truth for business table structure; `apply` idempotently syncs table schemas, menus, and `crud_log` adoption, and runs automatically at the tail of `migrate` so deployments stay `git pull && migrate`. Primary-key drift is refused with a pointer to business migrations, and `--allow-rebuild` is limited to disposable environments. See [`docs/crud-generation.md`](docs/crud-generation.md).
- Added a vite-style startup banner (Local/Network URLs and mode) printed after the listener is bound synchronously.
- Added `docker-compose.dev.yml` for building the image from local source; the compose service and image were renamed `app` → `buildadmin-go` (`DEPLOY_IMAGE_NAME` default updated).
- Fixed a remote panic in the public click-captcha endpoint: malformed coordinate payloads are now rejected as normal verification failures.
- Fixed permission caching: caches are now instance-owned and mutex-synchronized, and are invalidated after every rule/group/assignment mutation via after-commit hooks, so permission changes take effect immediately and rollbacks cannot leave stale invalidations.
- Fixed the shared QueryBuilder pagination bug: the offset was computed from the default limit before a custom limit was applied (`page=2&limit=20` now correctly yields offset 20).
- Fixed CRUD generator subdirectory packages (`generateRelativePath`) end-to-end: provider wiring, registrar type qualification, wire ProviderSet aggregation (generate/delete symmetry), subpackage model/handler naming, and `crud:delete` class-name derivation.
- Fixed `crud:delete` to be failure-safe: AST-based provider/registrar removal, a go/parser guard before the wire/build steps, and menu rule deletion moved after the build so DDL stays the last mutable step.
- Fixed the CRUD log status lifecycle: comment truncation for MySQL strict mode, stale `start` records reconciled as interrupted on the next generation lock, empty scaffold/directory pruning after delete, qualified registrar matching, and the mysql prefix hidden in the log table display.
- Fixed `RegisteredRoutes` collection to replace the snapshot atomically on each route collection instead of appending across router reinitializations.
- Improved Sortable performance: one ordered batch read and a single scoped CASE update replace per-row reads/updates (n+3 reads + n+1/n+2 writes → 3 reads + 2 writes).
- `make frontend` now syncs every top-level `web/dist` entry (`favicon.ico` was previously dropped), skips `pnpm install` when the lockfile is unchanged, and falls back to plain `pnpm install` when the lockfile is absent; the router now serves `/favicon.ico`.
- Internal: consolidated the duplicated QueryBuilder into `app/pkg/querybuilder` (type aliases keep existing and generated callers compiling), extracted the shared permission cache into `app/internal/permissioncache`, completed the terminal auth-model dependency inversion (`terminal.AuthModel`), consolidated the core schema inventory into a single ordered source, and dropped the `common` package's dependency on admin models.

### Upgrade

1. **`config.yaml` (required):** change `app.env` from `local` to `debug` (development) or `release` (production). The application refuses to start with the legacy `local` value.
2. **Country dictionary modules (only if your code references them):** update imports of the flat country packages to `app/admin/handler/country` / `app/admin/model/country`, and update any hard-coded `/admin/countryCurrency/*` / `/admin/countryLanguage*/*` API URLs (for example in custom frontend pages or external integrations) to the dotted form `/admin/country.Currency/*` etc. Non-super-admin permission checks for these modules now match the seeded `country/currency/...` button rules correctly.
3. **Docker deployments (only if used):** the compose service and image are now named `buildadmin-go`; update deployment scripts that targeted the old `app` service name and review `DEPLOY_IMAGE_NAME` in your `.env`.
4. **Deploy:** pull the release, rebuild frontend assets (`make frontend` or `pnpm build` from `web/`), then run `go run ./cmd/app --conf config.yaml migrate`. When a `crud_specs/` directory exists, `migrate` now finishes by idempotently applying your specs (table structure, menus, `crud_log` adoption), so no separate `crud:apply` invocation is needed in the normal deployment loop.

## v2.0.0

- **Breaking:** Removed the exported mutable auth cache globals `AuthGroupList`, `AuthRuleList`, and `AuthRuleNameList` from `app/admin/model` and `app/common/model`. Permission caches are now instance-owned, mutex-synchronized, and invalidated automatically on rule/group mutations; downstream code referencing those variables must drop the references (there is no replacement global API).
- **Breaking:** `Attachment.Admin` and `Attachment.User` in `app/common/model` now use the local `AttachmentAdmin`/`AttachmentUser` DTO types instead of `app/admin/model/simple.Admin`/`simple.User`. Field sets and JSON shapes are unchanged; downstream code naming the old types must switch to the new ones.
- **Breaking:** Introduced the RouteRegistrar routing system; `InitRouter` now receives a registrar set instead of the former 39 handwritten handler parameters.
- Changed the CRUD generator to produce module route registrar files and maintain `router/registrar_set.go`; it no longer injects route strings into `router/router.go`.
- Fixed a terminal security issue where a failed authentication check did not stop the configured command from executing.
- Corrected atomic capability key normalization and moved route collection after all registrar registrations.
- Added the 165-route golden snapshot baseline and runtime capability-to-route consistency tests.
- Added the downstream fork migration guide for converting custom modules to RouteRegistrar.

### Upgrade

See [`docs/route-registrar-migration.md`](docs/route-registrar-migration.md) for the downstream upgrade and module migration procedure.
