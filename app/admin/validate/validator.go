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
	if _, ok := err.(validator.ValidationErrors); ok {
		if len(err.(validator.ValidationErrors)) >= 1 {
			v := err.(validator.ValidationErrors)[0]
			if _, isValidator := data.(Validator); isValidator {
				// 若 data 结构体实现 Validator 接口即可实现自定义错误信息
				if message, exist := data.(Validator).GetMessages()[v.Field()+"."+v.Tag()]; exist {
					return cErr.BadRequest(message)
				}
			}
			return cErr.BadRequest(v.Error())
		}
	}
	return cErr.BadRequest(err.Error())
	// return cErr.ValidateErr("参数错误")
}
