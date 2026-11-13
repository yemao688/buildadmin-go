// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package router

import handler "buildadmin-go/internal/admin/handler"

const countryLanguageContentRoute = "country.LanguageContent"

func NewCountryLanguageContentRegistrar(h *handler.CountryLanguageContentHandler) *TableRegistrar {
	return NewTableRegistrar(countryLanguageContentRoute, h)
}
