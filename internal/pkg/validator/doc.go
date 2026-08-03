// Package validator 提供跨渠道共享的请求校验适配类型与工具（技术基建层）。
//
// 内容物：
//   - GetError/Validator/ValidatorMessages：handler 层校验错误转换，
//     支持结构体自定义错误文案（admin handler 约 19 个文件 + api/install
//     handler 引用）；
//   - Ids：批量删除等通用请求结构；
//   - Flex* 系列（FlexInt32/FlexInt64/FlexFloat64/FlexBool/FlexDateTime/
//     FlexDate/FlexClock/FlexYear/FlexFormattedUnixTime/FlexUnixTime 等）与
//     CommaJoined/KeyValueArray：CRUD 生成器产出的 DTO 与实体字段适配类型
//     （generator 的 FieldType override 会把这些类型写进 internal/model 实体）。
//
// 归属论证：本包由 internal/admin/validate 并入（用户裁定 validate 不应放在
// admin 渠道下，应归置技术基建层）。Flex 类型经生成器进入共享记录层
// internal/model 的实体字段，只有落在 pkg 层才能避免 model → 渠道反向依赖。
package validator
