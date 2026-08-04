package middleware

// NoNeedLoginer 由 handler 实现，声明哪些 action 不需要登录即可访问，
// 语义对齐 PHP 控制器的 $noNeedLogin 属性。路由注册时由渠道注册器
// （admin 的 RegisterHandlerExemptions、api 的 public 集合收集）读取。
type NoNeedLoginer interface {
	NoNeedLoginActions() []string
}

// NoNeedPermissioner 由 handler 实现，声明哪些 action 需要登录但不需要
// 权限校验（admin_rule），语义对齐 PHP 控制器的 $noNeedPermission 属性。
// 仅后台（/admin/*）存在权限模型；api 渠道不实现该接口。
type NoNeedPermissioner interface {
	NoNeedPermissionActions() []string
}
