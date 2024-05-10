package handler

import (
	"github.com/gin-gonic/gin"
)

type Base struct {
}

func (h *Base) Select(ctx *gin.Context) (map[string]interface{}, bool) {
	return nil, false
}

func (h *Base) CheckDataLimit(ctx *gin.Context, id int32) bool {
	value, _ := ctx.Get("dataLimitAdminIds")
	if value != nil {
		dataLimitAdminIds := value.([]int32)
		if len(dataLimitAdminIds) == 0 {
			return true
		}

		ok := false
		for _, v := range dataLimitAdminIds {
			if v == id {
				ok = true
				break
			}
		}
		return ok
	}
	return true
}

func (h *Base) Sortable(ctx *gin.Context, id int32) bool {
	return false
}

func Sortable[T any](ctx *gin.Context, m T) {

}
