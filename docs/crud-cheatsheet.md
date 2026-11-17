# CRUD 速查

任务导向的最短路径。契约细节（每个键、每种 designType、时间字段 JSON）见 [`crud-generation.md`](crud-generation.md)，本文只回答"我现在该敲什么"。

---

## 加一张业务表

```bash
go run ./cmd/crudgen crud:new ops_banner --comment 运营轮播   # 生成骨架（注释含冒号会自动加引号）
# 编辑 crud_specs/ops_banner.yaml，只补业务字段
go run ./cmd/crudgen crud:validate crud_specs/ops_banner.yaml
go run ./cmd/server --conf configs/config.yaml crud:generate crud_specs/ops_banner.yaml --dry-run  # 先看差量
go run ./cmd/server --conf configs/config.yaml crud:generate crud_specs/ops_banner.yaml
```

最小 spec（映射写法，键都可省）：

```yaml
name: ops_banner
comment: 轮播图表
quickSearchField: [title]
columnFields: [id, title, image, status, weigh, update_time, create_time]
fields:
  id: {type: bigint, unsigned: true, primaryKey: true, autoIncrement: true, designType: pk, comment: ID}
  title: {type: varchar, length: 100, comment: 标题}
  image: {type: varchar, length: 255, designType: image, comment: 图片}
  status: {type: tinyint, length: 1, default: "1", designType: switch, comment: "状态:0=禁用,1=启用"}
  weigh: {type: int, default: "0", designType: weigh, comment: 权重}
  create_time: {type: bigint, comment: 创建时间}
  update_time: {type: bigint, comment: 更新时间}
```

## 改字段 / 加字段

```bash
# 1. 改 crud_specs/<table>.yaml
# 2. 看差量（不写库、不写文件）
go run ./cmd/server --conf configs/config.yaml crud:diff crud_specs/<table>.yaml
# 3. 一条命令生成（type 省略即 auto：表存在走 alter）
go run ./cmd/server --conf configs/config.yaml crud:up crud_specs/<table>.yaml
```

- 输出里有 `requires-approval`：属正常变更（默认值、自增、类型扩宽、属性），按提示 `crud:apply <spec> --approve=<类别>` 或交给启动自动迁移（它按"全部已批准"执行）。
- 输出里有 `rejected`：**不会自动执行**。`crud:diff <spec> --write-migration` 会生成带逐条 TODO 的 business 迁移骨架，处理完存量数据再把 spec 对齐。

## 加归属（多级代理数据权限）

```yaml
dataScope:
  mode: auto          # 识别 admin_id 并套用层级范围
  reassignable: true  # 编辑时可改上级；列表自动出"上级代理"列
```

子表（订单、流水）冗余归属：

```yaml
dataScope:
  mode: auto
  inheritFrom: {table: seller_user, byColumn: seller_id}
```

登记是自动的（启动时扫 spec），**不要再手抄 owner 表清单**。删除管理员时的转移范围就是这些表。

## 加关联下拉

```yaml
user_id:
  type: int
  designType: remoteSelect
  comment: 会员
  form: {remoteTable: user, remotePk: id, remoteField: username, relationFields: username}
```

列表列写 `user_id`，原始 FK 列自动隐藏，改名后的关联列显示 `username`；搜索用 `quickSearchField: [user.username]`（点号条目只能手写，设计器不出）。

## 看模块与状态

```bash
crud:list                       # 表名/菜单/数据权限/字段数/继承来源
crud:list --json                # 同上，机器可读
crud:schema --out crud_specs/schema.json   # 刷新编辑器/AI 用的 JSON Schema
migrate status                  # 安装状态 + 三轨计数 + spec 数
migrate list                    # 只看业务轨（--all 三轨）
migrate --plan                  # 发版前检查 rejected 的唯一事实源
diagnose                        # 全部体检项一次输出（--json 可给 CI/agent）
```

## 报错怎么查

| 报错 | 处理 |
| --- | --- |
| `crud generation is forbidden for protected table` | 核心表（user/admin/...）只能 `registerOnly` 登记，不能生成 |
| `--skip-repo: repository file ... does not exist` | 首次生成不要带 `--skip-repo` |
| `refusing to overwrite existing CRUD output` | 产物与上次成功的 manifest 不一致：先 `crud:delete <table>` 或恢复原路径；`crud:delete` 会连 `crud_specs/<table>.yaml` 一起删，重生成前需 `git checkout` 该 spec |
| `type=create would drop and recreate` | 你显式写了 `type: create`。改成省略或 `alter`；确实要重建才加 `--allow-rebuild` |
| `no successful crud_log for table`（export-spec） | 该表从未成功生成过，直接手写 spec |
| `no successful CRUD generation manifest`（delete） | 现在会退化为「只删 spec」；若两者都没有则报错 |
| `unknown relation alias` | `quickSearchField` 的点号别名由外键字段名派生，且列必须在 `relationFields` 里 |

## 交付前检查

```bash
gofmt -l <changed files> && go build ./... && go test ./<affected packages> -run '<related>'
```

前端改动另在 `web/` 跑 `pnpm lint && pnpm test && pnpm typecheck && pnpm build`。发版前必须 `migrate --plan` 并把 `rejected` 影响逐条向用户确认。
