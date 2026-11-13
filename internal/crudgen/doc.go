// Package crudgen 实现 CRUD 模块的代码生成与 spec 物化（读 spec、写生成产物、按 spec 同步表结构）。
//
// 这一层是开发期代码生成工具，不属于 internal/pkg 技术基建：pkg 只放与业务渠道无关的技术原语，
// 而本包读 admin 仓库的元数据（information_schema 相关查询与菜单写入）来完成生成与 apply，
// 天然与后台渠道同侧。因此它单独位于 internal/crudgen，不被 pkg 依赖，也不被运行时业务路由依赖。
//
// 现状与约束：
//   - 当前直接 import internal/admin/repository 与 internal/admin/handler 读取元数据和写菜单，
//     这是已知的反向依赖，仅限生成期（CLI crud:*、setup、migrate 尾部 apply）；
//   - 新增代码不得把 admin 渠道的类型扩散到运行时路径，也不得让 internal/pkg 反向依赖本包；
//   - 生成产物内容与模板由本包决定，修改生成逻辑不属于纯迁移范围。
//
// 后续方向（端口倒置）：把元数据读取与菜单写入抽成本包定义的端口接口，由 admin 侧（或 commands 层）
// 提供实现并注入，届时本包对 admin 的 import 可全部移除。
package crudgen
