# 框架源仓库使用指南

本文面向使用 `buildadmin-go` 开发业务的普通开发者和 AI agent。本仓库是框架源仓库：`git@github.com:yemao688/buildadmin-go.git`，框架发布分支是 `v2`。

## 三方关系

- `upstream/v2`：框架源仓库的发布分支，只从这里获取框架更新。
- `origin`：用户在 GitHub 上 fork 后自己的仓库，用于保存和发布业务代码。
- `master`：用户 fork 中长期开发业务的分支。业务提交和框架升级合并结果都落在这里。

```text
框架源仓库
git@github.com:yemao688/buildadmin-go.git
              v2
               |
               | fetch / merge
               v
用户 fork: origin/master  <---- push
               ^
               |
          业务开发分支 master
```

标准方向是把 `upstream/v2` merge 到自己的 `master`。不要在 `master` 上 rebase `upstream/v2`，也不要 force push。

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
git fetch upstream v2 --tags
```

根据 fork 是否已有 `master` 选择一种场景。

**fork 已有 `master`：** 保留该分支及其业务历史，不要用 `v2` 覆盖它。如果本地尚未有该分支：

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
git switch -c master upstream/v2
git push -u origin master
```

此后长期在自己的 `master` 上开发，业务提交正常 push 到 `origin/master`。

## 本地安装与日常开发

### 环境

- Go 使用 `go.mod` 要求的 Go 1.25.x。
- 准备可连接的 MySQL。
- 前端在 `web/` 目录使用 pnpm；不要在根目录使用 npm。

### 首次安装

有两种方式，二选一。

**Web 安装：** 在仓库根目录启动后端：

```bash
air
# 或：go run ./cmd/app --conf config.yaml
```

浏览器访问 `http://127.0.0.1:9989/install`，按安装器填写 MySQL 和管理员信息。安装器会生成被 Git 忽略的 `conf/config.yaml` 并执行其中的迁移命令。

**手动配置和迁移：** 从模板复制运行配置，按目标环境填写数据库、密钥等值，再执行迁移：

```bash
cp conf/config.example.yaml conf/config.yaml
go run ./cmd/app --conf config.yaml migrate
```

`conf/config.yaml` 可能包含凭据，不要提交。迁移会修改数据库，执行前确认配置指向正确环境并做好备份。

前端日常开发必须在 `web/` 执行：

```bash
cd web
pnpm install --frozen-lockfile
pnpm dev
```

后端默认端口是 `9989`，Vite 默认端口是 `9988`。

### 业务 CRUD

生成业务模块前必须先阅读 [`crud-generation.md`](crud-generation.md)，在 `crud_specs/` 编写模块 YAML，然后使用生成链：

```bash
go run ./cmd/app --conf config.yaml crud:generate crud_specs/<module>.yaml [--skip-menu]
go build ./...
```

不要手写生成的 model、handler、provider、router、Wire 或 Vue 脚手架。删除生成模块使用：

```bash
go run ./cmd/app --conf config.yaml crud:delete <table_name>
```

删除命令不会 DROP 数据表；MySQL DDL 不可由文件回滚，生成和删除前都要确认数据库副作用。

## 标准框架升级流程

升级前确认当前在自己的 `master`，工作树干净，且已备份目标数据库。建议为每次升级建立临时分支，便于审查和回退：

```bash
git switch master
git status
git switch -c chore/merge-upstream-v2-YYYYMMDD
git fetch upstream v2 --tags
git log --oneline master..upstream/v2
git diff --stat master...upstream/v2
git merge upstream/v2
```

将 `YYYYMMDD` 替换为实际日期。`git merge` 产生冲突时，按下表处理；解决后检查 `git status`，逐个 `git add`，再执行 `git commit`。

合并完成后，使用目标环境的 `conf/config.yaml` 执行迁移。迁移有真实数据库副作用，先备份，并确认不是误连生产或其它共享数据库：

```bash
go run ./cmd/app --conf config.yaml migrate
```

按改动范围验证，不把 `go test ./...` 作为默认门槛：

```bash
# 根目录：运行受影响包的聚焦测试，并构建后端
go test ./path/to/package -run '^TestName$'
go build ./...

# 只有 provider 或 cmd/app/wire.go 等 Wire 来源变化时
go generate ./cmd/app
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
git merge --no-ff chore/merge-upstream-v2-YYYYMMDD
git push origin master
```

保留合并提交和升级记录；不要改写 `master` 的远端历史或 force push。

## 冲突处理速查

| 文件或区域 | 处理原则 |
|---|---|
| `router/router.go` | 保留框架和业务双方路由，合并注册位置并检查路径、权限和重复注册。 |
| provider 集合 | 合并双方 provider；来源解决后再按需要生成 Wire。 |
| `cmd/app/wire_gen.go` | 不手改。先解决 `wire.go`、provider 等来源，再运行 `go generate ./cmd/app`。 |
| `go.mod`、`go.sum` | 保留双方确需依赖，完成冲突处理后运行 `go mod tidy`，再构建和测试验证。 |
| `conf/config.example.yaml` | 以框架新增字段为基础；业务运行值放在被忽略的 `conf/config.yaml`，不要把凭据合入模板。 |
| 前端语言和生成文件 | 修改其来源文件或生成配置后重建，不直接保留冲突后的生成物；前端命令在 `web/` 用 pnpm。 |
| 迁移历史 | 绝不能改名、改 ID 或重写已有迁移。新增迁移解决兼容问题，并检查 official/local 注册表冲突。 |

## 业务代码边界

业务优先放在这些位置：新增 `crud_specs/`、生成并定制业务后端模块、`web/src/views/` 和 `web/src/lang/` 的业务前端、以及自己制定编号/命名策略的新迁移。下游迁移不必错误地占用框架预留编号；应使用独立且稳定的编号或命名空间，合并时检查 registry 冲突，并保证幂等、前缀安全。

以下区域尽量少改，以降低升级冲突：`cmd/app` wiring、`router/router.go` 的既有框架区域、`database/migrations/official/` 和 `local/` 的历史、`database/migrations/model/` 生成模型，以及 Docker/Makefile 等发布基础设施。业务确需扩展时，优先通过生成链和新增来源文件完成。

## AI agent 协议

1. 先读根目录 [`AGENTS.md`](../AGENTS.md) 和本指南，再确认真实入口、生成链和路由边界。
2. 开始前检查当前分支、remote 和工作树：`git branch --show-current`、`git remote -v`、`git status`。
3. CRUD 必须先读 [`crud-generation.md`](crud-generation.md)，写 `crud_specs/*.yaml`，再运行 `crud:generate`；删除使用 `crud:delete`。
4. 不手改 `cmd/app/wire_gen.go`、生成的 migration model 或其它 generated 文件；修改来源后重新生成。
5. 不修改历史迁移，不硬编码 `ba_` 表前缀；使用配置中的 `mysql.prefix`。
6. 不运行危险的 `go run ./cmd/generate`，除非明确检查其实现、DSN 和覆盖范围并得到专门确认。
7. 框架升级只能把 `upstream/v2` merge 到业务 `master`，不自行 rebase、force push、reset 或覆盖用户业务历史。
8. 按改动选择验证：受影响包聚焦测试、`go build ./...`、必要时 `go generate ./cmd/app`，前端在 `web/` 执行 pnpm typecheck/build。

## 危险或错误做法

| 做法 | 问题 |
|---|---|
| 在 `master` 上 rebase `upstream/v2` 或 force push | 改写共享业务历史，破坏 fork 协作。 |
| 脏工作树直接 merge | 容易把未完成业务改动混入升级冲突。 |
| 直接改 `wire_gen.go`、generated 文件或 migration model | 下次生成会覆盖，且来源问题仍未解决。 |
| 直接改名、改 ID 或重写历史迁移 | 破坏迁移账本和已有环境。 |
| 在 SQL 或代码中硬编码 `ba_` | 配置前缀可变，导致非默认前缀环境失败。 |
| 根目录运行 npm | 前端依赖和锁文件属于 `web/`，应使用 pnpm。 |
| 默认运行 `go test ./...` | 部分测试需要 MySQL 或特定 DI；先按影响范围聚焦验证。 |
| 例行运行 `go run ./cmd/generate` | 可能使用硬编码本地 MySQL DSN 并覆盖生成文件。 |
| 在容器内做首次 Web 安装 | 发布镜像不包含安装器流程；首次安装应在本地完成。 |

## 部署

部署流程请直接阅读 [`docs/docker-compose.md`](docker-compose.md)，本文不重复部署细节。
