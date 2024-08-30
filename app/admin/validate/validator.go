package validate

import (
	cErr "go-build-admin/app/pkg/error"

	"github.com/go-playground/validator/v10"
)

type Validator interface {
	GetMessages() ValidatorMessages
}

type ValidatorMessages map[string]string

// GetError 获取验证错误
func GetError(data interface{}, err error) *cErr.Error {
	validErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return cErr.BadRequest(err.Error())
	}

	if len(validErrs) == 0 {
		return cErr.BadRequest(err.Error())
	}

	if _, ok := data.(Validator); !ok {
		return cErr.BadRequest(validErrs[0].Error())
	}

	// 若 data 结构体实现 Validator 接口即可实现自定义错误信息
	if message, exist := data.(Validator).GetMessages()[validErrs[0].Field()+"."+validErrs[0].Tag()]; exist {
		return cErr.BadRequest(message)
	}

	return cErr.BadRequest(err.Error())
}
