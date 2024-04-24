package error

const (
	DefaultError  = 40000 // 默认错误
	ValidateError = 42200 // 验证错误
	TokenError    = 40100 // Token失效
	Forbidden     = 40300 // 无权限
	NotFoundData  = 40400 // 数据不存在
	UserNotFound  = 40401 // 用户不存在
	ServerError   = 50000 // 服务器错误
)
