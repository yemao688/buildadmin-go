package crud_helper

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/data_scope"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildSearchJoinLiteral 验证 remoteSelect 字段 → QueryBuilder SearchJoins
// 字面量构建：Alias 由字段名派生（relationNameForField，admin_id → admin、
// editor_id → editor，同表多 FK 各自独立别名）、mysql.prefix 拼接入 Table、
// RemotePk 缺省 id、仅 remoteSelect 参与（remoteSelects 的 CSV FK 不生成）、
// 按条目排序（首键即 Alias）保证可重复生成、无匹配返回空串。
func TestBuildSearchJoinLiteral(t *testing.T) {
	fields := []crudmodel.Field{
		{Name: "parent_admin_id", DesignType: "remoteSelect", Form: crudmodel.FormAttr{RemoteTable: "admin", RemotePk: "id"}},
		{Name: "reviewer_admin_ids", DesignType: "remoteSelects", Form: crudmodel.FormAttr{RemoteTable: "admin", RemotePk: "", RelationFields: "nickname,email"}},
		{Name: "title", DesignType: "string"},
		{Name: "user_id", DesignType: "remoteSelect", Form: crudmodel.FormAttr{RemoteTable: "user", RemotePk: "id"}},
	}
	got := buildSearchJoinLiteral(crudmodel.Table{}, fields, "ba_")
	want := `[]querybuilder.SearchJoin{{Alias: "parentAdmin", Table: "ba_admin", PK: "id", FK: "parent_admin_id"}, {Alias: "user", Table: "ba_user", PK: "id", FK: "user_id"}}`
	if got != want {
		t.Fatalf("buildSearchJoinLiteral() = %s; want %s", got, want)
	}

	// 排序稳定：字段顺序反转输出逐字节一致
	reversed := []crudmodel.Field{fields[3], fields[1], fields[0]}
	if again := buildSearchJoinLiteral(crudmodel.Table{}, reversed, "ba_"); again != got {
		t.Fatalf("buildSearchJoinLiteral() not stable across input order: %s != %s", again, got)
	}

	// 同表双 FK：admin_id + editor_id 都指向 admin 表 → 两条独立 Alias 不冲突
	// （对齐 PHP：relationName 从字段名派生，而非 RemoteTable 表名）
	dual := []crudmodel.Field{
		{Name: "admin_id", DesignType: "remoteSelect", Form: crudmodel.FormAttr{RemoteTable: "admin", RemotePk: "id"}},
		{Name: "editor_id", DesignType: "remoteSelect", Form: crudmodel.FormAttr{RemoteTable: "admin", RemotePk: "id"}},
	}
	if got := buildSearchJoinLiteral(crudmodel.Table{}, dual, "ba_"); got != `[]querybuilder.SearchJoin{{Alias: "admin", Table: "ba_admin", PK: "id", FK: "admin_id"}, {Alias: "editor", Table: "ba_admin", PK: "id", FK: "editor_id"}}` {
		t.Fatalf("buildSearchJoinLiteral() dual FK same table = %s; want distinct aliases", got)
	}

	// 纯 remoteSelects（CSV 多选 FK 无法等值关联）→ 空串
	if got := buildSearchJoinLiteral(crudmodel.Table{}, []crudmodel.Field{{Name: "reviewer_admin_ids", DesignType: "remoteSelects", Form: crudmodel.FormAttr{RemoteTable: "admin", RelationFields: "nickname,email"}}}, "ba_"); got != "" {
		t.Fatalf("buildSearchJoinLiteral() with only remoteSelects = %q; want empty", got)
	}

	// 无 remoteSelect 字段 → 空串（不生成赋值行，产物零 diff）
	if got := buildSearchJoinLiteral(crudmodel.Table{}, []crudmodel.Field{{Name: "title", DesignType: "string"}}, "ba_"); got != "" {
		t.Fatalf("buildSearchJoinLiteral() without remoteSelect = %q; want empty", got)
	}

	// RemoteTable 未配置 → 跳过
	if got := buildSearchJoinLiteral(crudmodel.Table{}, []crudmodel.Field{{Name: "user_id", DesignType: "remoteSelect"}}, "ba_"); got != "" {
		t.Fatalf("buildSearchJoinLiteral() without remoteTable = %q; want empty", got)
	}

	// 空 prefix → Table 不带前缀
	if got := buildSearchJoinLiteral(crudmodel.Table{}, []crudmodel.Field{{Name: "user_id", DesignType: "remoteSelect", Form: crudmodel.FormAttr{RemoteTable: "user"}}}, ""); got != `[]querybuilder.SearchJoin{{Alias: "user", Table: "user", PK: "id", FK: "user_id"}}` {
		t.Fatalf("buildSearchJoinLiteral() with empty prefix = %s; want unprefixed Table", got)
	}
}

// TestBuildRelationSearchColumn 验证 remoteSelect 关联搜索列：prop 为点号
// 形态（alias 由字段名派生，与关联显示列 prop/语言键同源）、operator 固定
// LIKE、语言键复用关联显示列的 relationFieldLangPrefix 约定
// （<小写 relationName>__<字段>）。
func TestBuildRelationSearchColumn(t *testing.T) {
	field := crudmodel.Field{
		Name: "user_id", DesignType: "remoteSelect",
		Form: crudmodel.FormAttr{RemoteTable: "user", RemotePk: "id", RelationFields: "username"},
	}
	got := buildRelationSearchColumn(field, "reviewer.")
	for _, want := range []string{
		`label: t("reviewer.user__username")`,
		`prop: "user.username"`,
		`align: "center"`,
		`operator: "LIKE"`,
		`operatorPlaceholder: t('Fuzzy query')`,
		`show: false`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildRelationSearchColumn() = %s; want contains %s", got, want)
		}
	}

	// 多关联字段取第一个（逗号分割 trim）
	multi := crudmodel.Field{
		Name: "user_id", DesignType: "remoteSelect",
		Form: crudmodel.FormAttr{RemoteTable: "user", RemotePk: "id", RelationFields: " nickname, email "},
	}
	if got := buildRelationSearchColumn(multi, ""); !strings.Contains(got, `prop: "user.nickname"`) {
		t.Fatalf("buildRelationSearchColumn() first relation field = %s; want prop user.nickname", got)
	}

	// 同表双 FK：editor_id 指向 admin 表 → prop 前缀为 editor（字段名派生，
	// 而非表名 admin），与后端 SearchJoin Alias 一致
	editor := crudmodel.Field{
		Name: "editor_id", DesignType: "remoteSelect",
		Form: crudmodel.FormAttr{RemoteTable: "admin", RemotePk: "id", RelationFields: "username"},
	}
	if got := buildRelationSearchColumn(editor, ""); !strings.Contains(got, `prop: "editor.username"`) {
		t.Fatalf("buildRelationSearchColumn() dual FK = %s; want prop editor.username", got)
	}
}

// TestModelSearchJoinLiteralRendering 验证仓库模板：有 SearchJoinLiteral 时
// 生成 tableInfo.SearchJoins 赋值并自动补 querybuilder import；三者皆空时
// 回退原 "QueryBuilder(ctx, s.TableInfo(), nil)" 形态（无关联表产物零 diff）。
func TestModelSearchJoinLiteralRendering(t *testing.T) {
	table := getTestTableData()
	table.DataScope = &data_scope.Config{Mode: data_scope.ModeNone}
	fields := getCompileFields("")
	getTableName := func(tableName string, fullName bool) string {
		prefix := ""
		if fullName {
			prefix = "ba_"
		}
		tableName = strings.TrimPrefix(tableName, prefix)
		return prefix + tableName
	}
	modelData, _, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, table.DataScope, getTableName, proveAll)
	require.NoError(t, err)
	modelData.Pk = "id"
	modelData.StructTemp = compileDemoStruct(modelData.ClassName, "", "", "")

	plain, err := renderModel(modelData)
	require.NoError(t, err)
	require.Contains(t, plain, "QueryBuilder(ctx, s.TableInfo(), nil)")
	require.NotContains(t, plain, "tableInfo.")

	data := modelData
	data.FieldTypesLiteral = `map[string]string{"userid": "bigint"}`
	data.SearchJoinLiteral = `[]querybuilder.SearchJoin{{Alias: "admin", Table: "ba_admin", PK: "id", FK: "admin_id"}}`
	withJoins, err := renderModel(data)
	require.NoError(t, err)
	require.Contains(t, withJoins, "tableInfo := s.TableInfo()")
	require.Contains(t, withJoins, `tableInfo.FieldTypes = map[string]string{"userid": "bigint"}`)
	require.Contains(t, withJoins, `tableInfo.SearchJoins = []querybuilder.SearchJoin{{Alias: "admin", Table: "ba_admin", PK: "id", FK: "admin_id"}}`)
	require.Contains(t, withJoins, `"buildadmin-go/internal/pkg/querybuilder"`)
	require.Contains(t, withJoins, "QueryBuilder(ctx, tableInfo, nil)")
	require.NotContains(t, withJoins, "QueryBuilder(ctx, s.TableInfo(), nil)")
}
