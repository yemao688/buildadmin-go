package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	repository "buildadmin-go/internal/admin/repository"
	model "buildadmin-go/internal/model"
	crudhooks "buildadmin-go/internal/pkg/crudhooks"
)

type CountryLanguageHandler struct {
	TableHandler[model.CountryLanguage, dto.CountryLanguageParam, int64]
	countryLanguageM *repository.CountryLanguageRepository
}

func NewCountryLanguageHandler(countryLanguageM *repository.CountryLanguageRepository) *CountryLanguageHandler {
	return &CountryLanguageHandler{
		TableHandler: NewTableHandler[model.CountryLanguage, dto.CountryLanguageParam, int64](
			countryLanguageM,
			TableOptions[model.CountryLanguage, dto.CountryLanguageParam]{
				PKJSONName:    "id",
				PartialEdit:   map[string]bool{},
				AfterMutation: crudhooks.Mutation("language-cache"),
			},
		),
		countryLanguageM: countryLanguageM,
	}
}
