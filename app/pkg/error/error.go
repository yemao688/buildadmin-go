package error

import "net/http"

type Error struct {
	httpCode  int
	errorCode int
	errorMsg  string
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
	errCode := DefaultError
	if len(errorCode) > 0 {
		errCode = errorCode[0]
	}
	return New(http.StatusBadRequest, errCode, errorMsg)
}

//需要token才能请求
func Unauthorized(errorMsg string, errorCode ...int) *Error {
	errCode := TokenError
	if len(errorCode) > 0 {
		errCode = errorCode[0]
	}
	return New(http.StatusOK, errCode, errorMsg)
}

//token请求的资源权限不够或非法
func ForbiddenRequest(errorMsg string, errorCode ...int) *Error {
	errCode := Forbidden
	if len(errorCode) > 0 {
		errCode = errorCode[0]
	}
	return New(http.StatusOK, errCode, errorMsg)
}

//数据不存在
func NotFound(errorMsg string) *Error {
	errCode := NotFoundData
	return New(http.StatusOK, errCode, errorMsg)
}

//参数验证错
func ValidateErr(errorMsg string) *Error {
	return New(http.StatusOK, ValidateError, errorMsg)
}

//内部错误 sql错误等
func InternalServer(errorMsg string) *Error {
	return New(http.StatusInternalServerError, ServerError, errorMsg)
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
