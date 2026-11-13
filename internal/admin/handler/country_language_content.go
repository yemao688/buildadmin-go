package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	repository "buildadmin-go/internal/admin/repository"
	model "buildadmin-go/internal/model"
)

type CountryLanguageContentHandler struct {
	TableHandler[model.CountryLanguageContent, dto.CountryLanguageContentParam, int64]
}

func NewCountryLanguageContentHandler(countryLanguageContentM *repository.CountryLanguageContentRepository) *CountryLanguageContentHandler {
	return &CountryLanguageContentHandler{
		TableHandler: NewTableHandler[model.CountryLanguageContent, dto.CountryLanguageContentParam, int64](
			countryLanguageContentM,
			TableOptions[model.CountryLanguageContent, dto.CountryLanguageContentParam]{
				PKJSONName:  "id",
				PartialEdit: map[string]bool{},
			},
		),
	}
}
