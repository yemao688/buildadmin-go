// Package crudgen 实现 CRUD 模块的代码生成与 spec 物化（读 spec、写生成产物、按 spec 同步表结构）。
//
// 这一层是开发期代码生成工具，不属于 internal/pkg 技术基建：pkg 只放与业务渠道无关的技术原语，
// 而本包承担生成期编排（CLI crud:*、setup、migrate 尾部 apply）。因此它单独位于 internal/crudgen，
// 不被 pkg 依赖，也不被运行时业务路由依赖。
//
// 依赖边界：
//   - 表元数据经本包定义的窄接口 TableStore 读取，实现是 internal/pkg/dbschema.Store，由
//     commands 层构造后作为参数注入；本包不 import internal/admin/*；
//   - 菜单与 crud_log 台账按表名经 GORM 直接写入（表名由配置前缀拼出），不依赖 admin 渠道类型；
//   - internal/boundary_test.go 的 R10 机械禁止本包 import admin/api/common/middleware，白名单为空；
//     新增代码不得绕过 TableStore 端口直接依赖渠道层；
//   - 不得让 internal/pkg 反向依赖本包；生成产物内容与模板由本包决定，修改生成逻辑不属于纯迁移范围。
package crudgen
