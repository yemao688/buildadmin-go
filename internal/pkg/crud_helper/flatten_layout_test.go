package crud_helper

import (
	"os"
	"path/filepath"
	"testing"

	"buildadmin-go/internal/pkg/util"

	"github.com/stretchr/testify/require"
)

// 拍平布局：文件恒为 <root>/<table>.go，历史 modelFile 前缀不再影响位置。
func TestParseFlatNameDataLocations(t *testing.T) {
	entity, err := ParseEntityNameData("country_language_content", "internal/admin/model/country/languageContent.go")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(util.RootPath(), "internal/model/country_language_content.go"), entity.ParseFile)
	require.Equal(t, "CountryLanguageContent", entity.LastName)
	require.Equal(t, "model", entity.Namespace)

	repo, err := ParseRepositoryNameData("country_language_content", "")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(util.RootPath(), "internal/admin/repository/country_language_content.go"), repo.ParseFile)
	require.Equal(t, "repository", repo.Namespace)
	require.Equal(t, "internal/admin/repository", repo.RootFileName)

	dto, err := ParseDTONameData("country_language_content", "")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(util.RootPath(), "internal/admin/dto/country_language_content.go"), dto.ParseFile)
	require.Equal(t, "dto", dto.Namespace)

	handler, err := ParseHandlerNameData("country_language_content", "")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(util.RootPath(), "internal/admin/handler/country_language_content.go"), handler.ParseFile)
	require.Equal(t, "handler", handler.Namespace)

	registrar, err := ParseRegistrarNameData("country_language_content", "")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(util.RootPath(), "internal/admin/router/country_language_content.go"), registrar.ParseFile)
	require.Equal(t, "router", registrar.Namespace)
}

// 多段 generateRelativePath 不再产生 Go 侧子目录（web views 行为不变）。
func TestParseFlatNameDataIgnoresMultiSegmentRelativePath(t *testing.T) {
	entity, err := ParseEntityNameData("ops_user_test_xxx", "internal/model/ops/user/test_xxx.go")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(util.RootPath(), "internal/model/ops_user_test_xxx.go"), entity.ParseFile)
	require.Equal(t, "OpsUserTestXxx", entity.LastName)
}

// ProvideRegistrars 锚点 code-mod 往返：真实 internal/admin/router/provider.go。
func TestProvideRegistrarsEntryRoundTrip(t *testing.T) {
	path := adminRouterProviderPath()
	original, err := os.ReadFile(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.WriteFile(path, original, 0644)) })

	// 添加（幂等）
	require.NoError(t, writeAdminRouterEntry("E2eBanner"))
	once, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(once), "\te2eBanner *handler.E2eBannerHandler,\n")
	require.Contains(t, string(once), "\t\tNewE2eBannerRegistrar(e2eBanner),\n")
	require.NoError(t, writeAdminRouterEntry("E2eBanner"))
	twice, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(once), string(twice), "anchor insertion is not idempotent")

	// 移除（幂等）
	require.NoError(t, removeAdminRouterEntry("E2eBanner"))
	removed, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(original), string(removed))
	require.NoError(t, removeAdminRouterEntry("E2eBanner"))
	removedAgain, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(original), string(removedAgain))
}

// F5: 锚点 code-mod 必须精确定位 ProvideRegistrars 函数体与真正的
// wire.NewSet ProviderSet；文件里其它同名结构不得被误操作。
func TestAnchorEditsOnlyTouchProvideRegistrarsAndWireNewSet(t *testing.T) {
	content := `package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAdminRouter,
)

var ProviderSetDup = wire.NewSet(
	NewImpostor,
)

func fakeReturn() []Registrar {
	return []Registrar{
		NewImpostorRegistrar(nil),
	}
}

func ProvideRegistrars(
	log *handler.LogHandler,
) []Registrar {
	return []Registrar{
		NewLogRegistrar(log),
	}
}
`
	// 追加只落在 ProvideRegistrars 的参数与 return
	added, err := addProvideRegistrarsEntry(content, "E2eBanner")
	require.NoError(t, err)
	require.Contains(t, added, "\te2eBanner *handler.E2eBannerHandler,\n")
	require.Contains(t, added, "\t\tNewE2eBannerRegistrar(e2eBanner),\n")
	require.NotContains(t, added, "NewImpostorRegistrar,\n")
	require.NotContains(t, added, "NewE2eBannerRegistrar,\n", "ProviderSet list must not gain the entry via addProvideRegistrarsEntry")
	// fakeReturn 不受影响
	require.Contains(t, added, "func fakeReturn() []Registrar {\n\treturn []Registrar{\n\t\tNewImpostorRegistrar(nil),\n\t}\n}")

	// ProviderSet 追加必须验证 callee 是 wire.NewSet；fake 的 ProviderSetDup 不动
	providerAdded, err := addProviderSetEntry(added, "E2eBannerRegistrar")
	require.NoError(t, err)
	require.Contains(t, providerAdded, "\tNewE2eBannerRegistrar,\n")
	require.Contains(t, providerAdded, "var ProviderSet = wire.NewSet(\n\tNewAdminRouter,\n\tNewE2eBannerRegistrar,\n)")
	require.NotContains(t, providerAdded, "NewImpostor,\n\tNewE2eBannerRegistrar", "ProviderSetDup must be untouched")

	// 移除：只删 ProvideRegistrars 相关 + 真正 wire.NewSet 里的条目
	removed, err := removeProvideRegistrarsEntry(providerAdded, "E2eBanner")
	require.NoError(t, err)
	require.NotContains(t, removed, "e2eBanner")
	require.NotContains(t, removed, "NewE2eBannerRegistrar")
	require.Contains(t, removed, "NewImpostorRegistrar(nil)")
	require.Contains(t, removed, "NewImpostor,")
	require.Equal(t, content, removed, "round trip must restore the original content")
}
