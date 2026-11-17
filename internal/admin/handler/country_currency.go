package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	repository "buildadmin-go/internal/admin/repository"
	model "buildadmin-go/internal/model"
	crudhooks "buildadmin-go/internal/pkg/crudhooks"
)

type CountryCurrencyHandler struct {
	TableHandler[model.CountryCurrency, dto.CountryCurrencyParam, int64]
	countryCurrencyM *repository.CountryCurrencyRepository
}

func NewCountryCurrencyHandler(countryCurrencyM *repository.CountryCurrencyRepository) *CountryCurrencyHandler {
	return &CountryCurrencyHandler{
		TableHandler: NewTableHandler[model.CountryCurrency, dto.CountryCurrencyParam, int64](
			countryCurrencyM,
			TableOptions[model.CountryCurrency, dto.CountryCurrencyParam]{
				PKJSONName:    "id",
				PartialEdit:   map[string]bool{},
				AfterMutation: crudhooks.Mutation("currency-cache"),
			},
		),
		countryCurrencyM: countryCurrencyM,
	}
}
