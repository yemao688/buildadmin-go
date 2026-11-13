package upload

import "errors"

// ErrMimetypeNotAllowed 表示上传文件类型不在配置白名单内。HTTP 层负责把
// 它映射为业务码 10002 与用户文案，共享领域层不感知 HTTP。
var ErrMimetypeNotAllowed = errors.New("The uploaded file format is not allowed")

// FileTooLargeError 携带已经按请求语言翻译好的超限文案。调用方负责映射
// 业务码；错误本身不依赖 gin 或 i18n。
type FileTooLargeError struct {
	Message string
}

func (e *FileTooLargeError) Error() string { return e.Message }

// BusinessMessage 返回上传领域错误对应的用户文案与是否命中；HTTP 层用它
// 映射业务码 10002，避免 common 直接依赖 cErr/HTTP。
func BusinessMessage(err error) (string, bool) {
	if errors.Is(err, ErrMimetypeNotAllowed) {
		return ErrMimetypeNotAllowed.Error(), true
	}
	var tooLarge *FileTooLargeError
	if errors.As(err, &tooLarge) {
		return tooLarge.Message, true
	}
	return "", false
}
