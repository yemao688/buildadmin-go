// Package validate 提供 admin 渠道共享的请求校验适配类型与工具。
//
// 内容物：
//   - GetError/Validator/ValidatorMessages：handler 层校验错误转换，
//     支持结构体自定义错误文案（internal/admin/handler 中约 19 个文件引用）；
//   - Ids：批量删除等通用请求结构；
//   - Flex* 系列（FlexInt32/FlexInt64/FlexFloat64/FlexBool/FlexDateTime/
//     FlexDate/FlexClock/FlexYear/FlexFormattedUnixTime/FlexUnixTime 等）与
//     CommaJoined/KeyValueArray：CRUD 生成器产出的 DTO 与实体字段适配类型
//     （generator 的 FieldType override 会把这些类型写进 internal/model 实体）。
//
// 归属论证（实证结论，防误删）：
//   - 非死代码：handler/dto/crud_helper 三处共 25+ 引用；
//   - 不能并入 internal/admin/dto：Flex 类型经生成器进入共享记录层
//     internal/model 的实体字段，并入渠道层 dto 会造成 model → admin 反向
//     依赖，违反分层边界（internal/boundary_test.go 机械执法）；
//   - 搬往 internal/pkg 是零语义收益的纯搬移，且需同步修改生成器输出契约。
package validate
