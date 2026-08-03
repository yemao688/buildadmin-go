# Go BuildAdmin

这是一个将 **BuildAdmin PHP 生态及其业务行为迁移到 Go** 的管理后台框架：后端使用 Go，前端基于 BuildAdmin v2.3.8。项目目标是让开发者和 AI agent 快速开发管理后台业务。它不是 PHP 的逐行翻译，而是在保持兼容性和业务语义的基础上，采用强类型的 Go、Gin、GORM 和 Wire 实现。

## 当前实现基线

- 全新安装建立 24 张表；后台管理功能完整（权限、安全、常规、字典、CRUD 代码生成）。
- 前台 `/` 是自包含占位页，提供最小 `userInfo` store 和 `/api/user/{login,register,logout}`，作为业务门户的开发起点。

## 技术栈与要求

- Go：以 `go.mod` 的 `go 1.25.x` 为准。
- 数据库：MySQL。
- 前端：Vue/Vite 8；Node 使用 Vite 8 支持的当前版本，不在此额外规定最低版本。
- 包管理：`pnpm`，不要使用 npm。
- 应用端口和时区：只认环境变量 `APP_PORT` 和 `APP_TIME_ZONE`，默认分别为 `9900` 和 `Asia/Shanghai`。启动时根目录缺少 `.env` 会自动从 `.env.example` 复制，已有环境变量不会被覆盖。
- 可选工具：Air 用于后端开发热重载。Wire 无需单独安装——`go generate ./cmd/server` 与 `crud:generate` 均通过 `go run` 按模块依赖运行 wire。

```bash
go install github.com/air-verse/air@latest
```

## 框架使用与升级

框架源仓库是 `git@github.com:yemao688/buildadmin-go.git`，发布分支为 `v2`。按你的角色选入口：

- **用框架开发业务**（大多数情况：fork 后在自己的 `master` 上长期开发）：先在 GitHub fork，自己的 fork 作为 `origin`，框架源仓库作为 `upstream`；框架更新时将 `upstream/v2` merge 到 `master`，验证后 push `origin/master`，不要 rebase 或 force push。完整的 fork、安装、CRUD 和升级流程见 [`docs/framework-workflow.md`](docs/framework-workflow.md)。
- **参与框架本身的开发**（在框架源仓库的 `v2` 分支上工作）：见 [`docs/framework-maintenance.md`](docs/framework-maintenance.md)。

以上两个文档同样约束 AI agent；AI 每会话先从 [`AGENTS.md`](AGENTS.md) 的"仓库身份自检"确认自己在哪一类仓库。

## 快速开始

1. 在项目根目录启动后端。端口和时区通过 `APP_PORT`、`APP_TIME_ZONE` 设置；缺少 `.env` 时启动会自动复制 `.env.example`：

   ```bash
   air
   # 或：go run ./cmd/server
   ```

   后端默认监听 `9900`。首次安装可访问 `http://127.0.0.1:9900/install` 使用 Web 向导，也可使用 CLI `go run ./cmd/server --conf configs/config.yaml setup`；手动迁移和安装边界见 [`docs/framework-workflow.md`](docs/framework-workflow.md)。如果让 AI 协助安装，请先向它提供数据库连接和管理员账号等必要信息，`configs/config.yaml` 交给安装器自动生成，不要手写（详见 `docs/framework-workflow.md` 的首次安装一节）。
2. 启动前端（必须在 `web/` 目录执行）：

   ```bash
   cd web
   pnpm install --frozen-lockfile
   pnpm dev
   ```

   Vite 默认监听 `9918` 并绑定 `0.0.0.0`，开发 API 地址默认使用 `VITE_AXIOS_BASE_URL` 指向 `APP_PORT=9900` 的 `http://localhost:9900`。

## Docker Compose 部署

发布机执行 `make frontend`（在 `web/` 构建并同步产物到根 `public/`），再执行 `make push`；Docker 只打包根 `public/`，不消费 `web/dist/`。生产机保存 `docker-compose.yaml`、`.env`（包含 `APP_PORT`/`APP_TIME_ZONE` 和发布变量）、`configs/config.yaml` 和 `runtime/`，然后执行 `docker compose pull && docker compose up -d`。首次安装在本地完成，详细流程（含本地开发镜像 `make run-docker-dev`）见 [`docs/docker-compose.md`](docs/docker-compose.md)。

## 常用命令

```bash
# 项目根目录
go build ./...
go test ./path/to/package -run '^TestName$'
go run ./cmd/server --conf configs/config.yaml migrate
go generate ./cmd/server                 # Wire 相关变更后

# web/ 目录
pnpm lint
pnpm typecheck
pnpm build
```

## 目录结构

```text
cmd/server/          应用入口（28 行极简 main，仅调用 commands.Execute）+ Wire wiring（wire.go/wire_gen.go）
internal/            全部私有代码（两业务渠道 + 安装渠道 + 共享内核）
  admin/             后台渠道：repository/dto/handler/router 均为单包（文件名=表名）、
                     middleware（登录/权限/安全审计）
  api/               门户/公共渠道：repository/service 均为单包（文件名=表/模块名）、
                     middleware（user_login）/dto/handler/router
  install/           安装渠道：handler + 自注册路由（/install 与 /api/install/*）+ provider
  model/             共享贫血实体记录层（含 projection/ 投影子包）
  common/            跨渠道领域服务（money/siteconfig/area/country/upload）
  pkg/               技术基建（persistence/data_scope/token/captcha/crud_helper/validator 等）
  middleware/        真·全局中间件
  router/            纯 bootstrap：引擎/全局中间件/静态资源/三渠道挂载
  migrations/        三轨迁移（official/framework/business）+ 内部迁移基础设施
  commands/          Cobra CLI 命令（root/server/crud/migrate/setup 等）
  conf/ infra/ i18n/ cron/  配置、连接初始化、本地化、定时任务
configs/config.defaults.yaml 运行基座（完整默认配置）
configs/config.yaml          配置覆盖层（忽略，不提交）
.env                 根目录运行环境与 Compose 变量（忽略，不提交）
internal/i18n/locales/ 本地化资源（YAML 语言包）
web/                 Vue/Vite 前端源码
public/              发布到镜像中的前端和运行时静态资源
crud_specs/          AI CRUD 生成 YAML
docs/                开发文档
runtime/             运行时日志和临时文件
```

## AI 驱动 CRUD 模块生成

生成业务模块前，先阅读 [`docs/crud-generation.md`](docs/crud-generation.md)，再将规范写入 `crud_specs/`，使用内置链路，不要手写实体、仓库、handler、registrar 或 Vue 脚手架：

```bash
go run ./cmd/server crud:validate crud_specs/<module>.yaml
go run ./cmd/server --conf configs/config.yaml crud:generate crud_specs/<module>.yaml [--skip-menu]
go run ./cmd/server --conf configs/config.yaml crud:delete <table_name>
```

生成失败时文件会自动恢复，但 MySQL DDL 不可回滚；执行前请检查数据库副作用和备份策略。

## 迁移、生成文件与测试注意事项

- 迁移采用 official/framework/business 三条轨道；framework 只有 `framework-final-seed-and-integrity`，三张台账为 `{prefix}migrations`、`{prefix}migrations_framework`、`{prefix}migrations_business`，统一使用五列。业务迁移、回滚和断点契约见 [`internal/migrations/business/README.md`](internal/migrations/business/README.md)。历史身份不可重写，迁移必须幂等、使用配置前缀，破坏性变更不能依赖 AutoMigrate。
- 不要手改 `cmd/server/wire_gen.go` 或自动生成的前端语言/类型文件；修改来源后重新生成。
- MySQL 集成测试由 `configs/config.yaml` 的 `mysql_test` 段驱动：开发机自建一次性测试库、对账号授予该库及 `<库名>%` 通配权限后设 `enabled: true`；未配置时相关测试统一提示并跳过，不会误动开发或生产库。细则见 [`AGENTS.md`](AGENTS.md)。

## 业务开发最佳实践

### 表设计：按业务分类加前缀

业务表用 `<分类>_<实体>` 命名，表、菜单和生成代码会自然归类，简单明了：

- 运营类：`ops_banner`、`ops_support`、`ops_help`
- 订单类：`order_recharge`、`order_withdraw`
- 用户类：`user_wallet`、`user_level`

多段式表名在 CRUD 规范里显式设置 `generateRelativePath`，标准值就是表名本身：首段是业务分类（也是 views 目录的分类段），其余段是实体名。Go 侧五类产物（实体/仓库/DTO/handler/路由注册器）全部按"文件名=表名"落单包（`internal/model`、`internal/admin/{repository,dto,handler,router}`），不再生成 `_route.go`；`generateRelativePath` 只决定 views 目录（lcfirst 驼峰化）与菜单/路由名（实体段 PascalCase，对齐 PHP 上游 URL 形态）。完整推导规则见 [`AGENTS.md`](AGENTS.md) 业务最佳实践一节，示例见 `crud_specs/country_language_content.yaml`。

**CRUD 模块按双 commit 工作流提交**：生成 commit 只含 spec 与全部生成产物（纯生成器输出，message 标注框架版本），业务微调一律独立 commit 并写明动机；重新生成后 `git diff` 对照微调 commit 逐条回补。完整规则与 round-trip 校验见 [`AGENTS.md`](AGENTS.md)。

### 业务迁移：只加文件，不动框架

业务表结构变更只写进 `internal/migrations/business/`，由 `Register`/`init` 注册；完整的台账、回滚和幂等契约见 [`internal/migrations/business/README.md`](internal/migrations/business/README.md)。不要把业务表加进 `official/` 或 `framework/`。

### 权限体系：直接在 admin 上建模，不要新建认证表

后台权限体系已经完整，多级代理/员工体系不需要新表：

- 超级管理员、总代理、代理、员工等角色 = `admin` 记录 + `admin_group` 角色组分配；
- 上下级关系 = `admin.parent_id`（配 `admin_closure` 闭包表，层级查询现成）；
- 数据权限隔离 = 业务表按需使用精确 `admin_id` 属主列，并由 CRUD `dataScope` 配置。

安全规则是全局资源，不通过规则表属主列隔离；安全日志中的 `admin_id` 只记录操作者。

`admin` 表字段允许微调：加业务字段、删掉用不到的字段都可以，但每个字段变更都要配一条 business 迁移（破坏性列变更不能依赖 AutoMigrate）。`user` 表同理：做前台会员业务时可以任意改造字段、删除闲置字段，并同步调整后台会员页面（`web/src/views/backend/user/`）。

### 前台随意改，后台不要动

- `web/src/views/frontend/`（用户端前台）当前是自包含占位页，**只是示例**，可以重构成任何业务门户样式；
- `web/src/views/backend/`（管理后台）**不要改样式**：保持与框架一致才能干净地合并后续框架更新；业务后台页面走 CRUD 生成，遵循生成器的既有模式。

## 鸣谢

感谢 [BuildAdmin](https://www.buildadmin.com/) 提供 PHP 生态和前端基础；本项目以前端 BuildAdmin v2.3.8 为基础并做了适配。
