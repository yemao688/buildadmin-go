package commands

import (
	"reflect"
	"strings"

	"buildadmin-go/internal/pkg/util"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// initValidator 注册 gin binding 的自定义验证器与 json tag 命名函数。
func initValidator() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		// 注册自定义验证器
		_ = v.RegisterValidation("phone", util.ValidatePhone)
		_ = v.RegisterValidation("password", util.ValidatePassword)

		// 注册自定义 json tag 函数
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	}
}
