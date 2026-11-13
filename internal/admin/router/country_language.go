// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package router

import handler "buildadmin-go/internal/admin/handler"

const countryLanguageRoute = "country.Language"

func NewCountryLanguageRegistrar(h *handler.CountryLanguageHandler) *TableRegistrar {
	return NewTableRegistrar(countryLanguageRoute, h)
}
