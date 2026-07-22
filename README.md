# Go BuildAdmin

这是一个将 **BuildAdmin PHP 生态及其业务行为迁移到 Go** 的管理后台框架：后端使用 Go，前端基于 BuildAdmin v2.3.7。项目目标是让开发者和 AI agent 快速开发管理后台业务。它不是 PHP 的逐行翻译，而是在保持兼容性和业务语义的基础上，采用强类型的 Go、Gin、GORM 和 Wire 实现。

## 技术栈与要求

- Go：以 `go.mod` 的 `go 1.25.x` 为准。
- 数据库：MySQL。
- 前端：Vue/Vite 8；Node 使用 Vite 8 支持的当前版本，不在此额外规定最低版本。
- 包管理：`pnpm`，不要使用 npm。
- 可选工具：Air 用于后端开发热重载；Wire 通过 `go generate ./cmd/app` 按项目声明运行。

```bash
go install github.com/air-verse/air@latest
```

## 框架使用与升级

本仓库是 `git@github.com:yemao688/buildadmin-go.git` 框架源仓库，发布分支为 `v2`。请先在 GitHub fork：自己的 fork 作为 `origin`，本仓库作为 `upstream`；业务长期开发在自己的 `master`。框架更新时，将 `upstream/v2` merge 到 `master`，验证后 push `origin/master`，不要 rebase 或 force push。完整的 fork、安装、CRUD 和升级流程见 [`docs/framework-workflow.md`](docs/framework-workflow.md)。

## 快速开始

1. 在项目根目录启动后端：

   ```bash
   air
   # 或：go run ./cmd/app --conf config.yaml
   ```

   后端默认监听 `9989`。
2. 浏览器打开 `http://127.0.0.1:9989/install`，按引导完成 Web 安装。安装器会从 `conf/config.example.yaml` 创建运行配置 `conf/config.yaml`，并执行其中配置的迁移命令。运行配置含凭据，不要提交。
3. 如果不使用 Web 安装器，可复制配置模板、按环境填写后直接执行数据库迁移：

   ```bash
   cp conf/config.example.yaml conf/config.yaml
   go run ./cmd/app --conf config.yaml migrate
   ```

4. 启动前端（必须在 `web/` 目录执行）：

   ```bash
   cd web
   pnpm install --frozen-lockfile
   pnpm dev
   ```

   Vite 默认监听 `9988`，开发 API 地址为 `http://localhost:9989`。

## Docker Compose 部署

发布入口：发布机执行 `make frontend`（在 `web/` 构建并同步产物到根 `static/`），再执行 `make push`；Docker 只打包根 `static/`，不消费 `web/dist/`。生产机保存 `docker-compose.yaml`、`.env`、`conf/config.yaml` 和 `storage/`，然后执行 `docker compose pull && docker compose up -d`。从 `conf/config.example.yaml` 复制生成 `conf/config.yaml`，并在应用 YAML 中设置 `app.time_zone`；`app.port` 保持 `9989`。首次安装在本地完成，详细流程见 [`docs/docker-compose.md`](docs/docker-compose.md)。

## 常用命令

```bash
# 项目根目录
go build ./...
go test ./path/to/package -run '^TestName$'
go run ./cmd/app --conf config.yaml migrate
go generate ./cmd/app                 # Wire 相关变更后

# web/ 目录
pnpm lint
pnpm typecheck
pnpm build
```

## 目录结构

```text
app/                 业务、命令、公共组件与中间件
cmd/app/             应用入口及 Wire wiring
router/              Gin 路由注册（/admin 与 /api）
database/migrations/ 三轨迁移（official/local/business）、迁移模型与内部迁移基础设施
conf/                配置模板和本地化资源
web/                 Vue/Vite 前端源码
static/              发布到镜像中的前端和运行时静态资源
crud_specs/          AI CRUD 生成 YAML
docs/                开发文档
tests/               测试支持代码
storage/             运行时上传文件和日志
```

## AI 驱动 CRUD 模块生成

生成业务模块前，先阅读 [`docs/crud-generation.md`](docs/crud-generation.md)，再将规范写入 `crud_specs/`，使用内置链路，不要手写 model、handler 或 Vue 脚手架：

```bash
go run ./cmd/app --conf config.yaml crud:generate crud_specs/<module>.yaml [--skip-menu]
go run ./cmd/app --conf config.yaml crud:delete <table_name>
```

生成失败时文件会自动恢复，但 MySQL DDL 不可回滚；执行前请检查数据库副作用和备份策略。

## 迁移、生成文件与测试注意事项

- 迁移采用三条轨道：`database/migrations/official/` 跟随上游 BuildAdmin 更新，`database/migrations/local/` 承载框架自身的 6 条语义迁移，`database/migrations/business/` 留给你注册业务迁移（独立 `business_migrations` 账本）。历史身份不可重写，迁移必须幂等、使用配置前缀，破坏性变更不能依赖 AutoMigrate。
- 不要手改 `cmd/app/wire_gen.go` 或自动生成的前端语言/类型文件；修改来源后重新生成。`go run ./cmd/generate` 可能使用硬编码本地 MySQL DSN，勿例行执行。
- MySQL 集成测试会修改数据库，需要 `BUILDADMIN_TEST_MYSQL_DSN` 和一次性数据库；测试覆盖和运行约束见 [`AGENTS.md`](AGENTS.md)。

## 业务开发最佳实践

### 表设计：按业务分类加前缀

业务表用 `<分类>_<实体>` 命名，表、菜单和生成代码会自然归类，简单明了：

- 运营类：`ops_banner`、`ops_support`、`ops_help`
- 订单类：`order_recharge`、`order_withdraw`
- 用户类：`user_wallet`、`user_level`

三段式表名（如 `country_language_content`）在 CRUD 规范里把最后一段写成驼峰：设置 `webViewsDir: country/languageContent`（参考 `crud_specs/country_language_content.yaml`），菜单名、视图目录和路由都保持单层简洁，而表名仍是可读的蛇形命名。

**每次 CRUD 生成后立刻提交一次 commit。** 生成会同时改动 model、handler、provider 装配、路由、菜单和 Vue 脚手架，一模一 commit 便于审查，也能保证 `crud:delete`/重新生成往返字节级一致；不要把手工改动混进生成提交。

### 业务迁移：只加文件，不动框架

业务表结构变更写进 `database/migrations/business/`：新增一个 Go 文件，在 `init()` 里调用 `business.Register(...)` 即可，编排器会自动发现并执行，记录到独立的 `business_migrations` 账本，与框架的 official/local 互不冲突。契约（幂等、前缀安全、按业务键判重等）见 [`database/migrations/business/README.md`](database/migrations/business/README.md)。不要把业务表加进 `official/` 或 `local/`。

### 权限体系：直接在 admin 上建模，不要新建认证表

后台权限体系已经完整，多级代理/员工体系不需要新表：

- 超级管理员、总代理、代理、员工等角色 = `admin` 记录 + `admin_group` 角色组分配；
- 上下级关系 = `admin.parent_id`（配 `admin_closure` 闭包表，层级查询现成）；
- 数据权限隔离 = 各业务表的 `admin_id` 属主列（框架迁移已建立并回填）。

`admin` 表字段允许微调：加业务字段、删掉用不到的字段都可以，但每个字段变更都要配一条 business 迁移（破坏性列变更不能依赖 AutoMigrate）。`user` 表同理：做前台会员业务时可以任意改造字段、删除闲置字段，并同步调整后台会员页面（`web/src/views/backend/user/`）。

### 前台随意改，后台不要动

- `web/src/views/frontend/`（用户端前台）**只是示例**，可以重构成任何业务门户样式，随便改；
- `web/src/views/backend/`（管理后台）**不要改样式**：保持与框架一致才能干净地合并后续框架更新；业务后台页面走 CRUD 生成，遵循生成器的既有模式。

## 鸣谢

感谢 [BuildAdmin](https://www.buildadmin.com/) 提供 PHP 生态和前端基础；本项目以前端 BuildAdmin v2.3.7 为基础并做了适配。
