package utils

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

// ValidatePhone 校验手机号
func ValidatePhone(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	ok, _ := regexp.MatchString(`^(13[0-9]|14[01456879]|15[0-35-9]|16[2567]|17[0-8]|18[0-9]|19[0-35-9])\d{8}$`, phone)
	return ok
}

// 校验登陆密码
func ValidatePassword(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if len(value) < 6 || len(value) > 32 {
		return false
	}
	regex := regexp.MustCompile(`^[^&<>"'\r\n]*$`)
	return regex.MatchString(value)
}
