package crud_helper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/utils"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// 拍平布局：文件恒为 <root>/<table>.go，历史 modelFile 前缀不再影响位置。
func TestParseFlatNameDataLocations(t *testing.T) {
	entity, err := ParseEntityNameData("country_language_content", "internal/admin/model/country/languageContent.go")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(utils.RootPath(), "internal/model/country_language_content.go"), entity.ParseFile)
	require.Equal(t, "CountryLanguageContent", entity.LastName)
	require.Equal(t, "model", entity.Namespace)

	repo, err := ParseRepositoryNameData("country_language_content", "")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(utils.RootPath(), "internal/admin/repository/country_language_content.go"), repo.ParseFile)
	require.Equal(t, "repository", repo.Namespace)
	require.Equal(t, "internal/admin/repository", repo.RootFileName)

	dto, err := ParseDTONameData("country_language_content", "")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(utils.RootPath(), "internal/admin/dto/country_language_content.go"), dto.ParseFile)
	require.Equal(t, "dto", dto.Namespace)

	handler, err := ParseHandlerNameData("country_language_content", "")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(utils.RootPath(), "internal/admin/handler/country_language_content.go"), handler.ParseFile)
	require.Equal(t, "handler", handler.Namespace)

	registrar, err := ParseRegistrarNameData("country_language_content", "")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(utils.RootPath(), "internal/admin/router/country_language_content.go"), registrar.ParseFile)
	require.Equal(t, "router", registrar.Namespace)
}

// 多段 generateRelativePath 不再产生 Go 侧子目录（web views 行为不变）。
func TestParseFlatNameDataIgnoresMultiSegmentRelativePath(t *testing.T) {
	entity, err := ParseEntityNameData("ops_user_test_xxx", "internal/model/ops/user/test_xxx.go")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(utils.RootPath(), "internal/model/ops_user_test_xxx.go"), entity.ParseFile)
	require.Equal(t, "OpsUserTestXxx", entity.LastName)
}

func TestClassifyDeleteLayout(t *testing.T) {
	root := utils.RootPath()
	flat := FileManifest{Generated: []string{
		filepath.Join(root, "internal/model/ops_e2e_banner.go"),
		filepath.Join(root, "internal/admin/repository/ops_e2e_banner.go"),
		filepath.Join(root, "internal/admin/dto/ops_e2e_banner.go"),
		filepath.Join(root, "internal/admin/handler/ops_e2e_banner.go"),
		filepath.Join(root, "internal/admin/router/ops_e2e_banner.go"),
	}}
	require.Equal(t, deleteLayoutFlat, classifyDeleteLayout(flat))

	nested := FileManifest{Generated: []string{
		filepath.Join(root, "internal/model/e2e_banner.go"),
		filepath.Join(root, "internal/admin/repository/ops/e2e_banner.go"),
		filepath.Join(root, "internal/admin/handler/ops/e2e_banner.go"),
		filepath.Join(root, "internal/admin/handler/ops/e2e_banner_route.go"),
	}}
	require.Equal(t, deleteLayoutNested, classifyDeleteLayout(nested))

	legacy := FileManifest{Generated: []string{
		filepath.Join(root, "internal/admin/model/ops/e2e_banner.go"),
		filepath.Join(root, "internal/admin/handler/ops/e2e_banner.go"),
	}}
	require.Equal(t, deleteLayoutLegacy, classifyDeleteLayout(legacy))
}

func TestDeriveNestedArtifacts(t *testing.T) {
	root := utils.RootPath()
	manifest := FileManifest{Generated: []string{
		filepath.Join(root, "internal/model/e2e_banner.go"),
		filepath.Join(root, "internal/admin/repository/ops/e2e_banner.go"),
		filepath.Join(root, "internal/admin/repository/ops/e2e_banner_custom.go"),
		filepath.Join(root, "internal/admin/handler/ops/e2e_banner.go"),
		filepath.Join(root, "internal/admin/handler/ops/e2e_banner_route.go"),
	}}
	className, handlerFile, repositoryFile := deriveNestedArtifacts(manifest)
	require.Equal(t, "E2eBanner", className)
	require.Equal(t, "internal/admin/handler/ops", handlerFile.RootFileName)
	require.Equal(t, "internal/admin/repository/ops", repositoryFile.RootFileName)
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

// 嵌套布局删除：manifest 记录旧路径时仍须干净删除 provider 条目、registrar
// 注册与生成文件。wire/编译阶段 stub（测试夹具的 provider 并不存在于真实
// wire 图，与 delete_failure_test 相同处理）。
func TestDeleteNestedLayoutManifest(t *testing.T) {
	originalWire := runWire
	originalBuild := runProjectBuild
	t.Cleanup(func() {
		runWire = originalWire
		runProjectBuild = originalBuild
	})
	runWire = func() error { return nil }
	runProjectBuild = func() error { return nil }

	db, cfg := newNestedDeleteFixture(t)
	require.NoError(t, DeleteFromSpec(db, cfg, "ops_e2e_banner"))
	requireNestedDeleteClean(t)
}

func newNestedDeleteFixture(t *testing.T) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "ba_", SingularTable: true}})
	require.NoError(t, err)
	cfg := &conf.Configuration{}
	cfg.Database.Prefix = "ba_"
	require.NoError(t, db.Exec(`CREATE TABLE ba_admin_rule (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		pid INTEGER NOT NULL DEFAULT 0,
		"type" TEXT NOT NULL DEFAULT 'menu',
		title TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL DEFAULT '',
		path TEXT NOT NULL DEFAULT '',
		icon TEXT NOT NULL DEFAULT '',
		menu_type TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL DEFAULT '',
		component TEXT NOT NULL DEFAULT '',
		keepalive INTEGER NOT NULL DEFAULT 0,
		extend TEXT NOT NULL DEFAULT 'none',
		remark TEXT NOT NULL DEFAULT '',
		weigh INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT '1',
		update_time INTEGER,
		create_time INTEGER
	)`).Error)
	require.NoError(t, db.Exec("CREATE TABLE ba_crud_log (id INTEGER PRIMARY KEY AUTOINCREMENT, admin_id INTEGER NOT NULL, table_name TEXT NOT NULL, `table` BLOB, fields BLOB, status TEXT NOT NULL, comment TEXT, connection TEXT NOT NULL, sync INTEGER, create_time INTEGER)").Error)

	root := utils.RootPath()
	repoDir := filepath.Join(root, "internal", "admin", "repository", "ops")
	handlerDir := filepath.Join(root, "internal", "admin", "handler", "ops")
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	require.NoError(t, os.MkdirAll(handlerDir, 0755))
	t.Cleanup(func() { _ = os.RemoveAll(repoDir) })
	t.Cleanup(func() { _ = os.RemoveAll(handlerDir) })

	repoProvider := filepath.Join(repoDir, "provider.go")
	handlerProvider := filepath.Join(handlerDir, "provider.go")
	require.NoError(t, os.WriteFile(repoProvider, []byte("package ops\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewKeepRepository,\n\tNewE2eBannerRepository,\n)\n"), 0644))
	require.NoError(t, os.WriteFile(handlerProvider, []byte("package ops\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewKeepHandler,\n\tNewE2eBannerHandler,\n\tNewE2eBannerRegistrar,\n)\n"), 0644))
	generated := []string{
		filepath.Join(root, "internal", "model", "e2e_banner.go"),
		filepath.Join(repoDir, "e2e_banner.go"),
		filepath.Join(handlerDir, "e2e_banner.go"),
		filepath.Join(handlerDir, "e2e_banner_route.go"),
	}
	for _, path := range generated {
		require.NoError(t, os.WriteFile(path, []byte("package fixture\n"), 0644))
	}
	t.Cleanup(func() {
		for _, path := range generated {
			_ = os.Remove(path)
		}
	})

	table := crudmodel.Table{
		Name:           "ops_e2e_banner",
		ModelFile:      "internal/model/e2e_banner.go",
		ControllerFile: filepath.ToSlash(filepath.Join("internal", "admin", "handler", "ops", "e2e_banner.go")),
		WebViewsDir:    "web/src/views/backend/ops/e2eBanner",
		Manifest: &crudmodel.CRUDFileManifest{
			Generated: generated,
			Shared:    []string{repoProvider, handlerProvider, filepath.Join(root, "cmd", "server", "wire.go"), filepath.Join(root, "cmd", "server", "wire_gen.go")},
		},
	}
	tableJSON, err := json.Marshal(table)
	require.NoError(t, err)
	fieldsJSON, err := json.Marshal([]crudmodel.Field{{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true, Unsigned: true}})
	require.NoError(t, err)
	require.NoError(t, db.Exec("INSERT INTO ba_crud_log (admin_id, table_name, `table`, fields, status, connection, sync, create_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", 1, "ops_e2e_banner", tableJSON, fieldsJSON, "success", "mysql", 0, 1).Error)
	return db, cfg
}

func requireNestedDeleteClean(t *testing.T) {
	t.Helper()
	root := utils.RootPath()
	repoProvider := filepath.Join(root, "internal", "admin", "repository", "ops", "provider.go")
	handlerProvider := filepath.Join(root, "internal", "admin", "handler", "ops", "provider.go")
	repo, err := os.ReadFile(repoProvider)
	require.NoError(t, err)
	require.NotContains(t, string(repo), "NewE2eBannerRepository")
	require.Contains(t, string(repo), "NewKeepRepository")
	handler, err := os.ReadFile(handlerProvider)
	require.NoError(t, err)
	require.NotContains(t, string(handler), "E2eBanner")
	require.Contains(t, string(handler), "NewKeepHandler")

	for _, path := range []string{
		filepath.Join(root, "internal", "model", "e2e_banner.go"),
		filepath.Join(root, "internal", "admin", "repository", "ops", "e2e_banner.go"),
		filepath.Join(root, "internal", "admin", "handler", "ops", "e2e_banner.go"),
		filepath.Join(root, "internal", "admin", "handler", "ops", "e2e_banner_route.go"),
	} {
		_, err := os.Stat(path)
		require.True(t, os.IsNotExist(err), "generated file %s still exists", path)
	}
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
