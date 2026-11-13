package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	repository "buildadmin-go/internal/admin/repository"
	model "buildadmin-go/internal/model"

	"go.uber.org/zap"
)

type CountryLanguageContentHandler struct {
	TableHandler[model.CountryLanguageContent, dto.CountryLanguageContentParam, int64]
	countryLanguageContentM *repository.CountryLanguageContentRepository
}

func NewCountryLanguageContentHandler(log *zap.Logger, countryLanguageContentM *repository.CountryLanguageContentRepository) *CountryLanguageContentHandler {
	return &CountryLanguageContentHandler{
		TableHandler: NewTableHandler[model.CountryLanguageContent, dto.CountryLanguageContentParam, int64](
			log,
			countryLanguageContentM,
			TableOptions[model.CountryLanguageContent, dto.CountryLanguageContentParam]{
				PKJSONName:  "id",
				PartialEdit: map[string]bool{},
			},
		),
		countryLanguageContentM: countryLanguageContentM,
	}
}
