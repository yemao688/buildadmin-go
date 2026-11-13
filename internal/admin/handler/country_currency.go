package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	repository "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/common/country"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/requesttx"

	"github.com/gin-gonic/gin"
)

type CountryCurrencyHandler struct {
	TableHandler[model.CountryCurrency, dto.CountryCurrencyParam, int64]
}

func NewCountryCurrencyHandler(countryCurrencyM *repository.CountryCurrencyRepository, countrySvc *country.Service) *CountryCurrencyHandler {
	return &CountryCurrencyHandler{
		TableHandler: NewTableHandler[model.CountryCurrency, dto.CountryCurrencyParam, int64](
			countryCurrencyM,
			TableOptions[model.CountryCurrency, dto.CountryCurrencyParam]{
				PKJSONName:  "id",
				PartialEdit: map[string]bool{},
				AfterMutation: func(ctx *gin.Context) {
					// 货币列表缓存失效：响应暂存成功后登记，请求事务提交后执行；非事务请求立即执行
					requesttx.InvalidateAfterMutation(ctx, countrySvc.InvalidateCurrencyCache)
				},
			},
		),
	}
}
