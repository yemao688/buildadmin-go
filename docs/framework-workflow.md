# 业务项目使用与升级指南

本文面向**业务仓库**（fork 自框架源仓库的业务项目）的普通开发者和 AI agent。框架源仓库是 `git@github.com:yemao688/buildadmin-go.git`，框架发布分支是 `v3`；参与框架本身开发请改读 [`framework-maintenance.md`](framework-maintenance.md)。

## 三方关系

- `upstream/v3`：框架源仓库的发布分支，只从这里获取框架更新。
- `origin`：用户在 GitHub 上 fork 后自己的仓库，用于保存和发布业务代码。
- `master`：用户 fork 中长期开发业务的分支。业务提交和框架升级合并结果都落在这里。

```text
框架源仓库
git@github.com:yemao688/buildadmin-go.git
              v3
               |
               | fetch / merge
               v
用户 fork: origin/master  <---- push
               ^
               |
          业务开发分支 master
```

标准方向是把 `upstream/v3` merge 到自己的 `master`。不要在 `master` 上 rebase `upstream/v3`，也不要 force push。

## 首次使用

### 1. Fork 和 clone

在 GitHub 打开 [buildadmin-go](https://github.com/yemao688/buildadmin-go)，点击 **Fork**，创建自己的 fork。然后 clone 自己的 fork：

```bash
git clone git@github.com:<your-account>/buildadmin-go.git
cd buildadmin-go
git remote add upstream git@github.com:yemao688/buildadmin-go.git
git remote -v
```

预期是 `origin` 指向自己的 fork，`upstream` 指向框架源仓库。若 `upstream` 已存在，不要重复添加，可用下面命令修正地址：

```bash
git remote set-url upstream git@github.com:yemao688/buildadmin-go.git
```

### 2. 初始化用户 `master`

先取得框架发布分支：

```bash
git fetch upstream v3 --tags
```

根据 fork 是否已有 `master` 选择一种场景。

**fork 已有 `master`：** 保留该分支及其业务历史，不要用 `v3` 覆盖它。如果本地尚未有该分支：

```bash
git switch --track origin/master
```

如果本地已经有 `master`，只需切换并确认工作树干净：

```bash
git switch master
git status
```

**fork 没有 `master`：** 从框架发布分支创建用户业务分支，再推送到自己的 fork：

```bash
git switch -c master upstream/v3
git push -u origin master
```

此后长期在自己的 `master` 上开发，业务提交正常 push 到 `origin/master`。

### 3. 创建 `AGENT_BUSINESS.md`（业务身份与协作说明）

在仓库根创建 `AGENT_BUSINESS.md`，声明本仓库是业务项目并记录业务协作规则。它是 AI agent 判断"当前是业务仓库"的首要标记（见根目录 `AGENTS.md` 的"仓库身份自检"）。复制模板、填写业务名后提交：

```bash
cp docs/templates/AGENT_BUSINESS.md AGENT_BUSINESS.md
# 编辑 AGENT_BUSINESS.md，替换 <业务名> 等占位
git add AGENT_BUSINESS.md && git commit -m "docs: declare project identity"
```

## 本地安装与日常开发

### 环境

- Go 使用 `go.mod` 要求的 Go 1.25.x。
- 准备可连接的 MySQL。
- 前端在 `web/` 目录使用 pnpm；不要在根目录使用 npm。

### 首次安装

安装统一使用 CLI `setup`（Web 安装向导已移除）：缺配置时启动任何命令都会打印 setup 指引并退出。

**交互式安装**（推荐）：在仓库根目录执行：

```bash
go run ./cmd/server setup
```

交互收集数据库连接、管理员信息并执行迁移与初始化。setup 尾部会同步 `crud_specs/*.yaml` 声明的业务表结构与菜单（与 `migrate` 尾部语义一致，不依赖 `crud.apply_on_migrate` 配置；无 spec 目录时自动跳过），全新安装开箱即含业务表。

**无人值守安装**（CI/容器首装）：配合全部 flags 一步完成：

```bash
go run ./cmd/server setup --yes \
  --db-host 127.0.0.1 --db-port 3306 --db-name buildadmin_go \
  --db-user root --db-password '密码' --db-prefix ba_ \
  --admin-name admin --admin-password '管理员密码' --site-name '站点名'
```

- 数据库不存在时 setup 会询问是否创建（`--yes` 下直接创建）；
- `--skip-frontend` 跳过前端构建（要求 `public/index.html` 已存在，适合 `make frontend` 已执行的环境）；省略则自动构建前端，Node/npm/包管理器缺失时会打印对应安装命令；
- 显式 `--conf` 路径会作为本次 setup 的配置文件路径。

**安装命令速查**（宿主机 / 容器内，参数见上）：

| 场景 | 命令 |
|---|---|
| 宿主机·交互式 | `make setup`（等价 `go run ./cmd/server setup`） |
| 宿主机·无人值守 | `make setup ARGS="--yes --db-host ... --db-name ... --db-user ... --db-password ... --admin-password ..."` |
| 宿主机·前端已构建 | 上面的 ARGS 追加 `--skip-frontend`（跳过构建，适合先跑过 `make frontend`） |
| 宿主机·先构建前端 | `make frontend` 后执行上一条 |
| 容器内·无人值守 | `make setup-docker ARGS="--yes --db-host mysql --db-name buildadmin_go --db-user ... --db-password ... --admin-password ... --skip-frontend"`（本地开发镜像，`--build` 自动以当前源码构建） |
| 容器内·生产镜像 | `docker compose run --rm buildadmin-go setup --yes --skip-frontend ...`（compose 启动前）或裸 `docker run --rm --network <mysql所在网络> -v ./configs:/app/configs -v ./runtime:/app/runtime -v ./public:/app/public <镜像> setup --yes ...` |
| 容器内·前端 | 镜像无 Node 工具链：先在宿主机 `make frontend`（产物进 `public/`），再以 `--skip-frontend` 安装；容器内 `up` 未安装会打印本指引并退出 |

**AI 协助安装：** 用户让 AI 帮忙安装时，AI 必须先向用户问询并收齐以下信息再开始执行，不要自行假设或先写配置：

1. MySQL 连接：主机、端口（默认 `3306`）、数据库名（不存在时安装器可创建）、用户名、密码；
2. 表前缀（默认 `ba_`）；
3. 管理员用户名与管理员密码（bcrypt 存储，安装后应立即修改）；
4. 站点名称（可选，有默认值）；
5. 前端是否立即构建（默认为是；CI/容器可用 `--skip-frontend`）；
6. 后端端口与时区不属 setup 收集范围，由 `.env` 的 `APP_PORT`/`APP_TIME_ZONE` 提供（默认 `9900`、`Asia/Shanghai`）。

`configs/config.yaml` 应由安装器自动生成——它只写 MySQL 连接和随机生成的 `token.key` 的稀疏覆盖层；不要手写 YAML（容易漏 `token.key`、格式出错或误提交凭据），也不要把生成的 `configs/config.yaml` 提交进仓库。setup 不会覆盖已存在的 `configs/config.yaml`；重装需先删除 `public/install.lock`。

**手动配置和迁移：** 创建只含目标环境覆盖值的 `configs/config.yaml`，填写数据库、密钥等值，再执行迁移。`configs/config.defaults.yaml` 是运行时完整基座，未写入覆盖层的键由它提供；`configs/config.yaml` 不需要复制完整基座，端口和时区仍通过 `.env` 中的 `APP_PORT`/`APP_TIME_ZONE` 设置：

```bash
go run ./cmd/server --conf configs/config.yaml migrate
```

根目录的 `configs/config.yaml` 可能包含凭据，不要提交。迁移会修改数据库，执行前确认配置指向正确环境并做好备份。

前端日常开发必须在 `web/` 执行：

```bash
cd web
pnpm install --frozen-lockfile
pnpm dev
```

后端默认使用 `APP_PORT=9900`，Vite 默认端口是 `9918` 并绑定 `0.0.0.0`；开发环境的 `VITE_AXIOS_BASE_URL` 默认指向 `http://localhost:9900`。修改后端端口时同步使用 `APP_PORT`，不要在 YAML 中设置 `app.port`。

### 业务 CRUD

生成业务模块前必须先阅读 [`crud-generation.md`](crud-generation.md)，在 `crud_specs/` 编写模块 YAML，然后使用生成链：

```bash
go run ./cmd/server crud:validate crud_specs/<module>.yaml
go run ./cmd/server --conf configs/config.yaml crud:generate crud_specs/<module>.yaml [--skip-menu]
go build ./...
```

不要手写生成的实体（`internal/model`）、仓库（`internal/admin/repository`）、handler、provider、router、Wire 或 Vue 脚手架。删除生成模块使用：

```bash
go run ./cmd/server --conf configs/config.yaml crud:delete <table_name>
```

删除命令不会 DROP 数据表；MySQL DDL 不可由文件回滚，生成和删除前都要确认数据库副作用。

### 后台路由权限

CRUD 生成器自动建规则无需处理；手写路由按同一规则二选一：声明 `middleware.RegisterPermissionExempt` 豁免，或通过 business 迁移补充 `admin_rule`，参考 [`framework-maintenance.md`](framework-maintenance.md) 的条款。

业务表迁移使用三轨台账中的 business 轨道；台账字段、断点列和 `migrate rollback` 语义以 [`internal/migrations/business/README.md`](../internal/migrations/business/README.md) 为准。

## 标准框架升级流程

升级前确认当前在自己的 `master`，工作树干净，且已备份目标数据库。建议为每次升级建立临时分支，便于审查和回退：

```bash
git switch master
git status
git switch -c chore/merge-upstream-v3-YYYYMMDD
git fetch upstream v3 --tags
git log --oneline master..upstream/v3
git diff --stat master...upstream/v3
git merge upstream/v3
```

将 `YYYYMMDD` 替换为实际日期。`git merge` 产生冲突时，按下表处理；解决后检查 `git status`，逐个 `git add`，再执行 `git commit`。

合并完成后，使用目标环境根目录的 `configs/config.yaml` 执行迁移。迁移有真实数据库副作用，先备份，并确认不是误连生产或其它共享数据库：

```bash
go run ./cmd/server --conf configs/config.yaml migrate
```

按改动范围验证，不把 `go test ./...` 作为默认门槛：

```bash
# 根目录：运行受影响包的聚焦测试，并构建后端
go test ./path/to/package -run '^TestName$'
go build ./...

# 只有 provider 或 cmd/server/wire.go 等 Wire 来源变化时
go generate ./cmd/server
go build ./...

# web/ 目录
cd web
pnpm install --frozen-lockfile
pnpm typecheck
pnpm build
```

确认 diff、迁移结果和测试结果后，把临时分支合并回自己的 `master`，再推送：

```bash
git switch master
git merge --no-ff chore/merge-upstream-v3-YYYYMMDD
git push origin master
```

保留合并提交和升级记录；不要改写 `master` 的远端历史或 force push。

## 冲突处理速查

| 文件或区域 | 处理原则 |
|---|---|
| `VERSION_FRAMEWORK`、`CHANGELOG.md` | 框架拥有的发行版本文件和变更记录；冲突时取框架侧版本。 |
| `VERSION` | 业务仓库自有的镜像/发布版本文件，框架永不提供；冲突时保留业务侧版本。 |
| `internal/router/router.go` | 纯 bootstrap（引擎/全局中间件/静态资源/三渠道挂载），框架基本独有；业务不应修改，路由一律走渠道 registrar。冲突时优先采用框架版本，再补业务 registrar。 |
| `internal/admin/repository/provider.go`、`internal/admin/handler/provider.go` | 双方都会在 `wire.NewSet` 中追加 `NewXxxRepository`/`NewXxxHandler` 构造器；保留两边新增条目，整理后运行 `go generate ./cmd/server`。 |
| `internal/admin/router/provider.go` | 持有合并 ProviderSet（`NewXxxRegistrar` 列表）与 `ProvideRegistrars` 锚点（每模块一行 handler 参数 + 返回条目）；冲突时两边条目都保留，来源解决后运行 `go generate ./cmd/server`。 |
| `internal/api/router/provider.go` | api 渠道 registrar 参数和 slice 条目（`ProvideRegistrars` 锚点）；冲突时两边条目都保留，整理后运行 `go generate ./cmd/server`。 |
| provider 集合 | 合并双方 provider；来源解决后再按需要生成 Wire。 |
| `cmd/server/wire_gen.go` | 永不手工解冲突。先解决 `wire.go`、各包 provider、`ProvideRegistrars` 等来源，再运行 `go generate ./cmd/server` 重生成。 |
| `go.mod`、`go.sum` | 保留双方确需依赖，完成冲突处理后运行 `go mod tidy`，再构建和测试验证。 |
| `configs/config.defaults.yaml` | 完整运行基座；框架新增字段在启动时自动可用。业务运行值放在根目录被忽略的稀疏 `configs/config.yaml` 覆盖层，不要把凭据合入基座。 |
| 前端语言和生成文件 | 修改其来源文件或生成配置后重建，不直接保留冲突后的生成物；前端命令在 `web/` 用 pnpm。 |
| 迁移历史 | 绝不能改名、改 ID 或重写已有迁移。新增迁移解决兼容问题，并检查 official/framework 注册表冲突。 |

## 业务版本约定

业务仓库在根目录 `VERSION` 文件维护自己的发布版本，版本格式由业务自行决定但应使用 semver。业务仓库还应在根目录 `AGENT_BUSINESS.md` 记录当前基于的框架版本，例如 `基于框架 v3.0.0`。框架仓库不提供也不维护 `VERSION` 文件。

## 业务代码边界

业务优先放在这些位置：新增 `crud_specs/`、生成并定制业务后端模块、`web/src/views/` 和 `web/src/lang/` 的业务前端、以及自己制定编号/命名策略的新迁移。下游迁移不必错误地占用框架预留编号；应使用独立且稳定的编号或命名空间，合并时检查 registry 冲突，并保证幂等、前缀安全。

路由注册走 RouteRegistrar 体系，业务路由不进 `internal/router/router.go`：admin 渠道每个模块由自己的 registrar 承载——文件恒为 `internal/admin/router/<table>.go`（`Group()` 声明分组、`Register(gin.IRoutes)` 注册路由、`Capabilities()` 声明原子能力），经 `internal/admin/router/provider.go` 的 `ProvideRegistrars` 锚点聚合（新模块一行 handler 参数 + 返回条目）后由 `AdminRouter` 挂载；api 渠道模块的 registrar 走 `internal/api/router/<module>.go`，经 `internal/api/router/provider.go` 的 `ProvideRegistrars` 锚点聚合后由 `ApiRouter` 挂载（安装不走 HTTP 渠道，统一由 CLI `setup` 完成）。CRUD 生成器自动产出 admin 渠道 registrar 并维护仓库/handler 的合并 ProviderSet 与 `ProvideRegistrars` 锚点，`crud:delete` 反向移除，不修改 `cmd/server/wire.go`。能力键保持既有协议：路由名由控制器与 action 组成，标准 CRUD action 为 `add`/`edit`/`del`，自定义 action 按原样保留。

以下区域尽量少改，以降低升级冲突：`cmd/server` wiring、`internal/router/router.go` 的既有框架区域、`internal/migrations/official/` 和 `internal/migrations/framework/` 的历史、`internal/model/` 的框架生成实体（驱动全新安装快照），以及 Docker/Makefile 等发布基础设施。业务确需扩展时，优先通过生成链和新增来源文件完成。

## AI agent 协议

1. 先读根目录 [`AGENTS.md`](../AGENTS.md) 完成仓库身份自检，再读本指南；确认真实入口、生成链和路由边界后再动手。
2. 开始前检查当前分支、remote 和工作树：`git branch --show-current`、`git remote -v`、`git status`。
3. CRUD 必须先读 [`crud-generation.md`](crud-generation.md)，写 `crud_specs/*.yaml`，再运行 `crud:generate`；删除使用 `crud:delete`。
4. 用户订单/充值等用户业务表默认用精确 `admin_id` 做 data-scope owner；`auto` 只识别精确 `admin_id`，不会把 `agent_admin_id` 当成默认 owner。默认 `columnFields` 保留有效 relation FK 以支持原始 ID 搜索，生成器会自动隐藏其 raw ID 列。
5. 不手改 `cmd/server/wire_gen.go`、生成的实体/仓库或其它 generated 文件；修改来源后重新生成。
6. 不修改历史迁移，不硬编码 `ba_` 表前缀；使用配置中的 `mysql.prefix`。
7. 框架升级只能把 `upstream/v3` merge 到业务 `master`，不自行 rebase、force push、reset 或覆盖用户业务历史。
8. 按改动选择验证：受影响包聚焦测试、`go build ./...`、必要时 `go generate ./cmd/server`，前端在 `web/` 执行 pnpm typecheck/build。

## 危险或错误做法

| 做法 | 问题 |
|---|---|
| 在 `master` 上 rebase `upstream/v3` 或 force push | 改写共享业务历史，破坏 fork 协作。 |
| 脏工作树直接 merge | 容易把未完成业务改动混入升级冲突。 |
| 直接改 `wire_gen.go`、generated 文件或 migration model | 下次生成会覆盖，且来源问题仍未解决。 |
| 直接改名、改 ID 或重写历史迁移 | 破坏迁移账本和已有环境。 |
| 在 SQL 或代码中硬编码 `ba_` | 配置前缀可变，导致非默认前缀环境失败。 |
| 根目录运行 npm | 前端依赖和锁文件属于 `web/`，应使用 pnpm。 |
| 默认运行 `go test ./...` | 部分测试需要 MySQL 或特定 DI；先按影响范围聚焦验证。 |
| 在容器内做首次安装（`docker compose run --rm buildadmin-go setup --yes ...`） | 需完整参数一次到位，不适合交互；建议本地完成安装后携带 `configs/config.yaml` 部署，或容器内用 `--yes` 无人值守安装。 |

## 部署

部署流程请直接阅读 [`docs/docker-compose.md`](docker-compose.md)，本文不重复部署细节。
