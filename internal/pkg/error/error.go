package error

import "net/http"

type Error struct {
	httpCode  int
	errorCode int
	errorMsg  string
}

func (e *Error) HttpCode() int {
	return e.httpCode
}

func (e *Error) ErrorCode() int {
	return e.errorCode
}

func (e *Error) Error() string {
	return e.errorMsg
}

func New(httpCode, errorCode int, errorMsg string) *Error {
	return &Error{
		httpCode:  httpCode,
		errorCode: errorCode,
		errorMsg:  errorMsg,
	}
}

//错误请求链接地址
func BadRequest(errorMsg string, errorCode ...int) *Error {
	errCode := http.StatusBadRequest
	if len(errorCode) > 0 {
		errCode = errorCode[0]
	}
	return New(http.StatusOK, errCode, errorMsg)
}

//未授权的
func Unauthorized(errorMsg string, errorCode ...int) *Error {
	errCode := http.StatusUnauthorized
	if len(errorCode) > 0 {
		errCode = errorCode[0]
	}
	return New(http.StatusOK, errCode, errorMsg)
}

//阻止请求的
func ForbiddenRequest(errorMsg string, errorCode ...int) *Error {
	errCode := http.StatusForbidden
	if len(errorCode) > 0 {
		errCode = errorCode[0]
	}
	return New(http.StatusOK, errCode, errorMsg)
}

//数据不存在
func NotFound(errorMsg string) *Error {
	errCode := http.StatusNotFound
	return New(http.StatusOK, errCode, errorMsg)
}

//内部错误 sql错误等
func InternalServer(errorMsg string) *Error {
	return New(http.StatusInternalServerError, ServerError, errorMsg)
}
