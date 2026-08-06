package crud_helper

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/util"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"text/template"

	"buildadmin-go/internal/pkg/data_scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func newField(name, typ string, pk bool) crudmodel.Field {
	return crudmodel.Field{Name: name, Type: typ, DesignType: typ, PrimaryKey: pk}
}

func proveAll(_ string) (bool, error)  { return true, nil }
func proveNone(_ string) (bool, error) { return false, nil }

func TestResolveDataScope_Matrix(t *testing.T) {
	cases := []struct {
		name         string
		cfg          *data_scope.Config
		fields       []crudmodel.Field
		allowNone    bool
		prove        func(string) (bool, error)
		wantMode     data_scope.Mode
		wantOwner    string
		wantGoField  string
		wantAssign   bool
		wantReassign bool
		wantIndex    IndexStrategy
		wantErr      bool
		errContains  string
	}{
		{
			name:        "legacy nil with exact admin_id fails closed without index proof",
			cfg:         nil,
			fields:      []crudmodel.Field{newField("id", "int", true), newField("admin_id", "int", false)},
			prove:       proveNone,
			wantErr:     true,
			errContains: "no proven index",
		},
		{
			name:        "legacy nil with exact admin_id and proven index",
			cfg:         nil,
			fields:      []crudmodel.Field{newField("id", "int", true), newField("admin_id", "int", false)},
			prove:       proveAll,
			wantMode:    data_scope.ModeAuto,
			wantOwner:   "admin_id",
			wantGoField: "AdminID",
			wantAssign:  true,
			wantIndex:   IndexProven,
		},
		{
			name:     "legacy nil with AdminID does not auto-detect",
			cfg:      nil,
			fields:   []crudmodel.Field{newField("id", "int", true), newField("AdminID", "int", false)},
			prove:    proveAll,
			wantMode: data_scope.ModeNone,
		},
		{
			name:     "auto with last_admin_id is not auto owner",
			cfg:      &data_scope.Config{Mode: data_scope.ModeAuto},
			fields:   []crudmodel.Field{newField("id", "int", true), newField("last_admin_id", "int", false)},
			prove:    proveAll,
			wantMode: data_scope.ModeNone,
		},
		{
			name:        "required custom owner last_admin_id without proven index fails",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "last_admin_id"},
			fields:      []crudmodel.Field{newField("id", "int", true), newField("last_admin_id", "int", false)},
			prove:       proveNone,
			wantErr:     true,
			errContains: "no proven index",
		},
		{
			name:        "required custom owner operator_admin_id with proven index",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "operator_admin_id", AssignOnCreate: ptr(true)},
			fields:      []crudmodel.Field{newField("id", "int", true), newField("operator_admin_id", "int", false)},
			prove:       proveAll,
			wantMode:    data_scope.ModeRequired,
			wantOwner:   "operator_admin_id",
			wantGoField: "OperatorAdminID",
			wantAssign:  true,
			wantIndex:   IndexProven,
		},
		{
			name:        "required missing owner column",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired},
			fields:      []crudmodel.Field{newField("id", "int", true)},
			wantErr:     true,
			errContains: "owner column is required",
		},
		{
			name:        "required owner column wrong type",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "owner_name"},
			fields:      []crudmodel.Field{newField("id", "int", true), newField("owner_name", "varchar", false)},
			wantErr:     true,
			errContains: "not integer-compatible",
		},
		{
			name:        "required custom owner with assign on create",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "owner_id", AssignOnCreate: ptr(true)},
			fields:      []crudmodel.Field{newField("id", "int", true), newField("owner_id", "int", false)},
			prove:       proveAll,
			wantMode:    data_scope.ModeRequired,
			wantOwner:   "owner_id",
			wantGoField: "OwnerID",
			wantAssign:  true,
			wantIndex:   IndexProven,
		},
		{
			name:        "none override with admin_id requires explicit flag",
			cfg:         &data_scope.Config{Mode: data_scope.ModeNone},
			fields:      []crudmodel.Field{newField("id", "int", true), newField("admin_id", "int", false)},
			prove:       proveAll,
			wantErr:     true,
			errContains: "explicit override",
		},
		{
			name:      "none override with admin_id allowed when explicit",
			cfg:       &data_scope.Config{Mode: data_scope.ModeNone},
			fields:    []crudmodel.Field{newField("id", "int", true), newField("admin_id", "int", false)},
			allowNone: true,
			prove:     proveAll,
			wantMode:  data_scope.ModeNone,
		},
		{
			name:        "admin.id explicit required",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "id"},
			fields:      []crudmodel.Field{newField("id", "int", true)},
			wantMode:    data_scope.ModeRequired,
			wantOwner:   "id",
			wantGoField: "ID",
			wantAssign:  false,
			wantIndex:   IndexProven,
		},
		{
			name:        "admin.id cannot assign on create",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "id", AssignOnCreate: ptr(true)},
			fields:      []crudmodel.Field{newField("id", "int", true)},
			wantErr:     true,
			errContains: "admin.id cannot assign on create",
		},
		{
			name:         "auto reassignable with admin_id",
			cfg:          &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true},
			fields:       []crudmodel.Field{newField("id", "int", true), newField("admin_id", "int", false)},
			prove:        proveAll,
			wantMode:     data_scope.ModeAuto,
			wantOwner:    "admin_id",
			wantGoField:  "AdminID",
			wantAssign:   true,
			wantReassign: true,
			wantIndex:    IndexProven,
		},
		{
			name:        "reassignable without owner fails",
			cfg:         &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true},
			fields:      []crudmodel.Field{newField("id", "int", true)},
			prove:       proveAll,
			wantErr:     true,
			errContains: "requires an owner column",
		},
		{
			name:        "reassignable required without assignOnCreate fails",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "admin_id", Reassignable: true},
			fields:      []crudmodel.Field{newField("id", "int", true), newField("admin_id", "int", false)},
			prove:       proveAll,
			wantErr:     true,
			errContains: "reassignable requires assignOnCreate=true",
		},
		{
			name:        "reassignable non-admin owner column rejected",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "agent_id", AssignOnCreate: ptr(true), Reassignable: true},
			fields:      []crudmodel.Field{newField("id", "int", true), newField("agent_id", "int", false)},
			prove:       proveAll,
			wantErr:     true,
			errContains: "requires owner column admin_id",
		},
		{
			name:        "reassignable bigint owner rejected (int32 contract)",
			cfg:         &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true},
			fields:      []crudmodel.Field{newField("id", "int", true), newField("admin_id", "bigint", false)},
			prove:       proveAll,
			wantErr:     true,
			errContains: "must be int32-compatible",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveDataScope(tc.cfg, tc.fields, DataScopeResolveOptions{
				AllowNoneWithAdminID: tc.allowNone,
				ProveIndex:           tc.prove,
			})
			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantMode, got.Policy.Mode)
			assert.Equal(t, tc.wantOwner, got.OwnerColumn)
			assert.Equal(t, tc.wantGoField, got.OwnerGoField)
			assert.Equal(t, tc.wantAssign, got.AssignOnCreate)
			assert.Equal(t, tc.wantReassign, got.Policy.Reassignable)
			assert.Equal(t, tc.wantIndex, got.IndexStrategy)
		})
	}
}

func TestResolveDataScope_NoProverFailsClosed(t *testing.T) {
	fields := []crudmodel.Field{newField("id", "int", true), newField("admin_id", "int", false)}
	_, err := ResolveDataScope(nil, fields, DataScopeResolveOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot prove an index")
}

func TestResolveDataScope_ReadExtraOwnersValidation(t *testing.T) {
	fields := []crudmodel.Field{newField("id", "int", true), newField("admin_id", "int", false), newField("secondary_admin_id", "bigint", false)}
	resolved, err := ResolveDataScope(&data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "admin_id", ReadExtraOwners: []string{"secondary_admin_id"}, AssignOnCreate: ptr(true)}, fields, DataScopeResolveOptions{ProveIndex: proveAll})
	require.NoError(t, err)
	assert.Equal(t, []string{"secondary_admin_id"}, resolved.Policy.ReadExtraOwners)

	for _, extra := range []string{"missing_admin_id", "seller.admin_id", "admin_id"} {
		_, err = ResolveDataScope(&data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "admin_id", ReadExtraOwners: []string{extra}, AssignOnCreate: ptr(true)}, fields, DataScopeResolveOptions{ProveIndex: proveAll})
		assert.Error(t, err, extra)
	}
}

func TestResolveDataScope_UserOwnedSpecDefaultsToExactAdminIDOnly(t *testing.T) {
	adminIDFields := []crudmodel.Field{
		newField("id", "bigint", true),
		newField("user_id", "bigint", false),
		newField("admin_id", "bigint", false),
	}
	resolved, err := ResolveDataScope(&data_scope.Config{Mode: data_scope.ModeAuto}, adminIDFields, DataScopeResolveOptions{ProveIndex: proveAll})
	require.NoError(t, err)
	assert.Equal(t, data_scope.ModeAuto, resolved.Policy.Mode)
	assert.Equal(t, "admin_id", resolved.OwnerColumn)
	assert.True(t, resolved.AssignOnCreate)

	agentOwnerFields := []crudmodel.Field{
		newField("id", "bigint", true),
		newField("user_id", "bigint", false),
		newField("agent_admin_id", "bigint", false),
	}
	resolved, err = ResolveDataScope(&data_scope.Config{Mode: data_scope.ModeAuto}, agentOwnerFields, DataScopeResolveOptions{ProveIndex: proveAll})
	require.NoError(t, err)
	assert.Equal(t, data_scope.ModeNone, resolved.Policy.Mode)
	assert.Empty(t, resolved.OwnerColumn)
	assert.False(t, resolved.AssignOnCreate)
}

func TestModelTemplate_DataScopeAuto(t *testing.T) {
	out := renderModelString(t, ModelData{
		Namespace:             "model",
		Name:                  "demo",
		ClassName:             "Demo",
		ModelVar:              "demo",
		Pk:                    "id",
		PkGoField:             "ID",
		DataScopePolicy:       data_scope.ResourcePolicy{Mode: data_scope.ModeAuto, OwnerColumn: "admin_id", AssignOnCreate: true},
		DataScopeOwnerGoField: "AdminID",
		EditableColumns:       []string{"name"},
		EditableColumnsGo:     `"name"`,
	})

	assert.Contains(t, out, "Enforcer data_scope.Enforcer")
	assert.NotContains(t, out, "func (s *DemoModel) scopedDB")
	assert.Contains(t, out, "func (s *DemoRepository) ScopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB")
	assert.Contains(t, out, "data_scope.OwnerRef{TableAlias: s.TableName, Column: s.Policy.OwnerColumn}")
	assert.Contains(t, out, "demo.AdminID = int32(actor.AdminID)")
	assert.Contains(t, out, `Where("id = ?", demo.ID)`)
	assert.Contains(t, out, "RowsAffected")
	assert.Contains(t, out, "case 0:")
	assert.Contains(t, out, `tx.Table(s.TableName).Model(&model.Demo{}).Where("id = ?", demo.ID).Count(&visible).Error`)
	assert.Contains(t, out, `return fmt.Errorf("unexpected edit rows affected: %d", res.RowsAffected)`)
	assert.NotContains(t, out, "LimitAdminIds")
	assert.NotContains(t, out, ".Save(&demo)")
}

func TestModelTemplate_ReadExtraOwners(t *testing.T) {
	out := renderModelString(t, ModelData{
		Namespace: "model",
		Name:      "demo",
		ClassName: "Demo",
		ModelVar:  "demo",
		Pk:        "id",
		PkGoField: "ID",
		DataScopePolicy: data_scope.ResourcePolicy{
			Mode:            data_scope.ModeRequired,
			OwnerColumn:     "admin_id",
			ReadExtraOwners: []string{"seller_id", "hotel_id"},
		},
		DataScopeOwnerGoField: "AdminID",
		EditableColumns:       []string{"name"},
		EditableColumnsGo:     `"name"`,
	})

	assert.Contains(t, out, `ReadExtraOwners: []string{"seller_id", "hotel_id"}`)
	assert.Contains(t, out, "func (s *DemoRepository) readScopedDB(ctx *gin.Context, db *gorm.DB) *gorm.DB")
	assert.Contains(t, out, "return data_scope.ScopeRead(ctx, db, s.Enforcer, data_scope.OwnerRef{TableAlias: s.TableName, Column: s.Policy.OwnerColumn}, extras)")
	assert.Contains(t, out, "db := s.readScopedDB(ctx, s.DBFor(ctx)).Session(&gorm.Session{})")
	assert.Contains(t, out, "countDB := s.readScopedDB(ctx, s.DBFor(ctx)).Session(&gorm.Session{})")
	assert.Contains(t, out, "findDB := s.readScopedDB(ctx, s.DBFor(ctx)).Session(&gorm.Session{})")

	editStart := strings.Index(out, "func (s *DemoRepository) Edit")
	require.GreaterOrEqual(t, editStart, 0)
	assert.Contains(t, out[editStart:], "tx = s.scopeDB(ctx, tx)")
}

func TestModelTemplate_EditNoOpUsesGeneratedPrimaryKeyAndScopedDB(t *testing.T) {
	out := renderModelString(t, ModelData{
		Namespace:         "model",
		Name:              "order_item",
		ClassName:         "OrderItem",
		ModelVar:          "orderItem",
		Pk:                "order_id",
		PkGoField:         "OrderId",
		DataScopePolicy:   data_scope.ResourcePolicy{Mode: data_scope.ModeRequired, OwnerColumn: "owner_id"},
		EditableColumns:   []string{"name"},
		EditableColumnsGo: `"name"`,
	})

	editStart := strings.Index(out, "func (s *OrderItemRepository) Edit")
	if editStart < 0 {
		t.Fatal("generated Edit method is missing")
	}
	edit := out[editStart:]
	assert.Contains(t, edit, "tx = s.scopeDB(ctx, tx)")
	assert.Contains(t, edit, `tx.Table(s.TableName).Model(&model.OrderItem{}).Where("order_id = ?", orderItem.OrderId).Count(&visible).Error`)
	assert.NotContains(t, edit, `Where("id = ?", orderItem.ID)`)
}

func TestModelTemplate_UsesLogicalSnakeCaseTableName(t *testing.T) {
	out := renderModelString(t, ModelData{
		Namespace:         "model",
		Name:              "order_item",
		ClassName:         "OrderItem",
		ModelVar:          "orderItem",
		Pk:                "id",
		PkGoField:         "ID",
		DataScopePolicy:   data_scope.ResourcePolicy{Mode: data_scope.ModeNone},
		EditableColumns:   []string{"name"},
		EditableColumnsGo: `"name"`,
	})

	assert.Contains(t, out, `config.Database.Prefix+"order_item"`)
	assert.NotContains(t, out, `"orderItem"`)
}

func TestGeneratedBigIntPrimaryKeyCompiles(t *testing.T) {
	table := getTestTableData()
	table.Name = "big_order"
	table.ModelFile = "internal/admin/model/BigOrder.go"
	table.ControllerFile = "internal/admin/handler/BigOrder.go"
	fields := []crudmodel.Field{
		{Name: "order_id", Type: "bigint", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}
	getTableName := func(name string, full bool) string {
		if full {
			return "ba_" + name
		}
		return name
	}
	modelData, handlerData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, &data_scope.Config{Mode: data_scope.ModeNone}, getTableName, proveAll)
	require.NoError(t, err)
	modelData.Pk = "order_id"
	className := modelData.ClassName
	modelData.StructTemp = strings.Replace(
		compileDemoStruct(className, "", "", ""),
		"\tID int32 `gorm:\"column:id;primaryKey;autoIncrement:true\" json:\"id\"`\n",
		"\tOrderId int64 `gorm:\"column:order_id;primaryKey\" json:\"order_id\"`\n",
		1,
	)
	modelCode, err := renderModel(modelData)
	require.NoError(t, err)
	handlerCode, err := renderHandler(handlerData)
	require.NoError(t, err)
	assert.Contains(t, modelCode, "GetOne(ctx *gin.Context, id int64)")
	assert.Contains(t, modelCode, `Where("order_id=?", id)`)
	assert.Contains(t, modelCode, "normalize"+className+"IDs(ids interface{}) ([]int64, error)")
	require.NoError(t, compileDataScopeFixture(t, className, modelCode, handlerCode, modelData.StructTemp, handlerData))
}

func TestRelatedModelWithIDAndNameCompilesWithEditableName(t *testing.T) {
	table := crudmodel.Table{Name: "ai_gate_base", ModelFile: "internal/admin/model/AiGateBase.go", ControllerFile: "internal/admin/handler/AiGateBase.go", FormFields: []string{"name"}, ColumnFields: []string{"id", "name"}}
	fields := []crudmodel.Field{
		{Name: "id", Type: "bigint", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}
	getTableName := func(name string, full bool) string {
		if full {
			return "ba_" + name
		}
		return name
	}
	modelData, handlerData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, nil, getTableName, proveAll)
	require.NoError(t, err)
	modelData.Pk = "id"
	modelData.StructTemp = compileDemoStruct(modelData.ClassName, "", "", "")
	modelCode, err := renderModel(modelData)
	require.NoError(t, err)
	assert.Contains(t, modelCode, `Select("name", "update_time")`)
	assert.NotContains(t, modelCode, ".Select()")
	handlerCode, err := renderHandler(handlerData)
	require.NoError(t, err)
	require.NoError(t, compileDataScopeFixture(t, modelData.ClassName, modelCode, handlerCode, modelData.StructTemp, handlerData))
}

func TestBigIntOwnerUsesInt64ActorConversion(t *testing.T) {
	table := crudmodel.Table{Name: "big_owner", ModelFile: "internal/admin/model/BigOwner.go", ControllerFile: "internal/admin/handler/BigOwner.go", FormFields: []string{"name"}}
	fields := []crudmodel.Field{
		{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "admin_id", Type: "bigint", DesignType: "number"},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}
	getTableName := func(name string, full bool) string {
		if full {
			return "ba_" + name
		}
		return name
	}
	modelData, handlerData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, nil, getTableName, proveAll)
	require.NoError(t, err)
	assert.Equal(t, "int64", modelData.DataScopeOwnerGoType)
	modelData.Pk = "id"
	modelData.StructTemp = strings.Replace(compileDemoStruct(modelData.ClassName, "admin_id", "AdminID", "admin_id"), "AdminID int32", "AdminID int64", 1)
	modelCode, err := renderModel(modelData)
	require.NoError(t, err)
	assert.Contains(t, modelCode, "AdminID = int64(actor.AdminID)")
	handlerCode, err := renderHandler(handlerData)
	require.NoError(t, err)
	require.NoError(t, compileDataScopeFixture(t, modelData.ClassName, modelCode, handlerCode, modelData.StructTemp, handlerData))
}

func TestGeneratedStringPrimaryKeyBatchDeleteUsesStrings(t *testing.T) {
	out := renderModelString(t, ModelData{
		Namespace: "model", Name: "token", ClassName: "Token", ModelVar: "token",
		Pk: "token", PkGoType: "string", PkGoField: "Token", DataScopePolicy: data_scope.ResourcePolicy{Mode: data_scope.ModeNone},
		EditableColumns: []string{"name"}, EditableColumnsGo: `"name"`,
	})
	assert.Contains(t, out, "normalizeTokenIDs(ids interface{}) ([]string, error)")
	assert.Contains(t, out, "raw, ok := ids.([]string)")
}

func TestValidateGenerationInputRejectsTextPrimaryKey(t *testing.T) {
	err := ValidateGenerationInput(crudmodel.Table{Name: "text_key"}, []crudmodel.Field{{Name: "token", Type: "text", PrimaryKey: true}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported primary key type")
}

func TestRelatedModelWithoutAdminIDResolvesModeNone(t *testing.T) {
	resolved, err := ResolveDataScope(nil, []crudmodel.Field{{Name: "id", Type: "int", PrimaryKey: true}, {Name: "name", Type: "varchar"}}, DataScopeResolveOptions{ProveIndex: proveAll})
	require.NoError(t, err)
	assert.Equal(t, data_scope.ModeNone, resolved.Policy.Mode)
}

func TestCommonModelBaseSupportsRequestTransactions(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "pkg", "persistence", "base.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "func (s *BaseModel) DBFor")
	assert.Contains(t, string(content), "func (s *BaseModel) Transaction")
}

func TestRequiredOwnerWithoutAssignOnCreateIsRejected(t *testing.T) {
	_, err := ResolveDataScope(&data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "owner_id", AssignOnCreate: ptr(false)}, []crudmodel.Field{{Name: "id", Type: "int", PrimaryKey: true}, {Name: "owner_id", Type: "int"}}, DataScopeResolveOptions{ProveIndex: proveAll})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "assignOnCreate=true")
}

func TestPrepareGenerationData_SharedEntityImportPath(t *testing.T) {
	table := crudmodel.Table{
		Name:           "order_item",
		ModelFile:      "internal/common/model/OrderItem.go", // 历史前缀仍可解析出类名
		ControllerFile: "internal/admin/handler/OrderItem.go",
	}
	fields := []crudmodel.Field{{Name: "id", Type: "int", PrimaryKey: true}}
	getTableName := func(name string, full bool) string {
		if full {
			return "ba_" + name
		}
		return name
	}

	_, handlerData, entityFile, repositoryFile, dtoFile, handlerFile, registrarFile, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, &data_scope.Config{Mode: data_scope.ModeNone}, getTableName, proveAll)
	require.NoError(t, err)
	assert.Equal(t, "buildadmin-go/internal/model", handlerData.ModelImportPath)
	assert.Equal(t, "OrderItem", entityFile.LastName)
	// 拍平布局：文件恒为 <root>/<table>.go
	assert.Equal(t, filepath.Join(util.RootPath(), "internal", "model", "order_item.go"), entityFile.ParseFile)
	assert.Equal(t, "internal/admin/repository", repositoryFile.RootFileName)
	assert.Equal(t, "internal/admin/dto", dtoFile.RootFileName)
	assert.Equal(t, "internal/admin/handler", handlerFile.RootFileName)
	assert.Equal(t, "internal/admin/router", registrarFile.RootFileName)
}

func TestModelTemplate_DataScopeNone(t *testing.T) {
	out := renderModelString(t, ModelData{
		Namespace:         "model",
		Name:              "demo",
		ClassName:         "Demo",
		ModelVar:          "demo",
		Pk:                "id",
		PkGoField:         "ID",
		DataScopePolicy:   data_scope.ResourcePolicy{Mode: data_scope.ModeNone},
		EditableColumns:   []string{"name"},
		EditableColumnsGo: `"name"`,
	})

	assert.Contains(t, out, "Policy   data_scope.ResourcePolicy")
	assert.Contains(t, out, `Mode:            "none"`)
	assert.Contains(t, out, "s.readScopedDB(ctx, s.DBFor(ctx))")
	assert.Contains(t, out, "return s.scopeDB(ctx, db)")
	assert.NotContains(t, out, "actor.AdminID")
	assert.NotContains(t, out, "LimitAdminIds")
	assert.NotContains(t, out, ".Save(&demo)")
	// 非级联表输出不变性：Policy 字面量仅多 InheritFrom: nil 一行；
	// 不生成 CascadeOwners() 方法，Edit 无级联循环，不 import clause。
	assert.Contains(t, out, "InheritFrom:     nil,")
	assert.NotContains(t, out, "CascadeOwners()")
	assert.NotContains(t, out, "// @cascade:begin")
	assert.NotContains(t, out, "for _, c := range s.CascadeOwners()")
	assert.NotContains(t, out, `"gorm.io/gorm/clause"`)
}

func TestModelTemplate_UsesRequestTransactions(t *testing.T) {
	out := renderModelString(t, ModelData{
		Namespace: "model", Name: "demo", ClassName: "Demo", ModelVar: "demo",
		Pk: "id", PkGoField: "ID", DataScopePolicy: data_scope.ResourcePolicy{Mode: data_scope.ModeNone},
		EditableColumns: []string{"name"}, EditableColumnsGo: `"name"`,
	})

	assert.Contains(t, out, "s.DBFor(ctx)")
	assert.Contains(t, out, "return s.Transaction(ctx, func(tx *gorm.DB) error")
	assert.Contains(t, out, "tx = s.scopeDB(ctx, tx)")
	assert.NotContains(t, out, "s.sqlDB.Begin")
	assert.NotContains(t, out, ".Session(&gorm.Session{}).Begin")
}

func TestModelTemplate_NilEnforcerFailClosed(t *testing.T) {
	out := renderModelString(t, ModelData{
		Namespace:             "model",
		Name:                  "demo",
		ClassName:             "Demo",
		ModelVar:              "demo",
		Pk:                    "id",
		PkGoField:             "ID",
		DataScopePolicy:       data_scope.ResourcePolicy{Mode: data_scope.ModeAuto, OwnerColumn: "admin_id", AssignOnCreate: true},
		DataScopeOwnerGoField: "AdminID",
		EditableColumns:       []string{"name"},
		EditableColumnsGo:     `"name"`,
	})
	assert.Contains(t, out, "tx.AddError(data_scope.ErrScopedAccessDenied)")
}

func TestExcludeParamFields_RemovesOwnerAndId(t *testing.T) {
	input := "type DemoParam struct {\n" +
		"\tID int32 `gorm:\"column:id\" json:\"id\"`\n" +
		"\tAdminID int32 `gorm:\"column:admin_id\" json:\"admin_id\"`\n" +
		"\tName string `gorm:\"column:name\" json:\"name\"`\n" +
		"}"
	out := excludeParamFields(input, []string{"admin_id"})
	assert.NotContains(t, out, `json:"admin_id"`)
	assert.NotContains(t, out, `json:"id"`)
	assert.Contains(t, out, `json:"name"`)
}

func TestCrudLogTableDataScopeRoundtrip(t *testing.T) {
	cfg := &data_scope.Config{
		Mode:        data_scope.ModeRequired,
		OwnerColumn: "operator_admin_id",
	}
	table := crudmodel.Table{
		Name:       "demo",
		DataScope:  cfg,
		FormFields: []string{"name", "operator_admin_id"},
	}

	data, err := json.Marshal(table)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"dataScope":`)
	assert.Contains(t, string(data), `"mode":"required"`)
	assert.Contains(t, string(data), `"ownerColumn":"operator_admin_id"`)

	var decoded crudmodel.Table
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.NotNil(t, decoded.DataScope)
	assert.Equal(t, data_scope.ModeRequired, decoded.DataScope.Mode)
	assert.Equal(t, "operator_admin_id", decoded.DataScope.OwnerColumn)

	// Legacy nil DataScope resolves to auto when fed through generation.
	legacy := crudmodel.Table{Name: "legacy"}
	legacyData, err := json.Marshal(legacy)
	require.NoError(t, err)
	var legacyDecoded crudmodel.Table
	require.NoError(t, json.Unmarshal(legacyData, &legacyDecoded))
	assert.Nil(t, legacyDecoded.DataScope)
}

func TestEffectiveFormFieldsReachPopupFormRender(t *testing.T) {
	cases := []struct {
		name       string
		cfg        *data_scope.Config
		owner      string
		wantOwner  bool
		ownerField crudmodel.Field
	}{
		{
			name:  "auto admin_id",
			owner: "admin_id",
			ownerField: crudmodel.Field{
				Name: "admin_id", Type: "int", DesignType: "number",
			},
		},
		{
			name:  "custom owner",
			cfg:   &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "operator_admin_id", AssignOnCreate: ptr(true)},
			owner: "operator_admin_id",
			ownerField: crudmodel.Field{
				Name: "operator_admin_id", Type: "int", DesignType: "number",
			},
		},
		{
			name:  "admin.id",
			cfg:   &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "id"},
			owner: "id",
			ownerField: crudmodel.Field{
				Name: "id", Type: "int", DesignType: "number", PrimaryKey: true,
			},
		},
		{
			name:       "explicit global retains admin_id",
			cfg:        &data_scope.Config{Mode: data_scope.ModeNone},
			owner:      "admin_id",
			wantOwner:  true,
			ownerField: crudmodel.Field{Name: "admin_id", Type: "int", DesignType: "number"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table := crudmodel.Table{
				Name:           "form_scope_test",
				Comment:        "form scope",
				ModelFile:      "internal/admin/model/FormScopeTest.go",
				ControllerFile: "internal/admin/handler/FormScopeTest.go",
				WebViewsDir:    "web/src/views/backend/form_scope_test",
				FormFields:     []string{"name", tc.owner},
				DataScope:      tc.cfg,
			}
			fields := []crudmodel.Field{
				{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
				{Name: "name", Type: "varchar", DesignType: "string"},
				tc.ownerField,
			}
			getTableName := func(name string, full bool) string {
				if full {
					return "ba_" + name
				}
				return name
			}
			modelData, _, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, tc.cfg, getTableName, proveAll)
			require.NoError(t, err)
			assert.NotNil(t, modelData.EffectiveFormFields)
			ownerInEffective := slices.Contains(modelData.EffectiveFormFields, tc.owner)
			assert.Equal(t, tc.wantOwner, ownerInEffective)

			formMarkup := buildFormFieldMarkup(modelData.EffectiveFormFields, fields, "form_scope.", getTableName)
			formContent, err := renderFormFile(FormVueData{BigDialog: "false", FormFields: formMarkup}, fields, "form_scope.")
			require.NoError(t, err)
			if tc.wantOwner {
				assert.Contains(t, formContent, "form_scope."+tc.owner)
			} else {
				assert.NotContains(t, formContent, "form_scope."+tc.owner)
			}
			assert.Contains(t, formContent, "form_scope.name")
		})
	}
}

func TestGeneratedDataScopeCompiles(t *testing.T) {
	cases := []struct {
		name        string
		cfg         *data_scope.Config
		ownerCol    string
		ownerGo     string
		assign      bool
		structOwner string
	}{
		{
			name:        "auto_admin_id",
			cfg:         nil,
			ownerCol:    "admin_id",
			ownerGo:     "AdminID",
			assign:      true,
			structOwner: "admin_id",
		},
		{
			name:        "explicit_global_none_with_admin_id",
			cfg:         &data_scope.Config{Mode: data_scope.ModeNone},
			ownerCol:    "",
			ownerGo:     "",
			assign:      false,
			structOwner: "admin_id",
		},
		{
			name:        "required_custom_owner",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "operator_admin_id", AssignOnCreate: ptr(true)},
			ownerCol:    "operator_admin_id",
			ownerGo:     "OperatorAdminID",
			assign:      true,
			structOwner: "operator_admin_id",
		},
		{
			name:        "required_admin_dot_id",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "id"},
			ownerCol:    "id",
			ownerGo:     "ID",
			assign:      false,
			structOwner: "id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table := getTestTableData()
			table.DataScope = tc.cfg
			fields := getCompileFields(tc.structOwner)

			getTableName := func(name string, full bool) string {
				prefix := ""
				if full {
					prefix = "ba_"
				}
				return prefix + strings.TrimPrefix(name, prefix)
			}

			modelData, handlerData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, tc.cfg, getTableName, proveAll)
			require.NoError(t, err)

			assert.Equal(t, tc.ownerCol, modelData.DataScopePolicy.OwnerColumn)
			assert.Equal(t, tc.assign, modelData.DataScopePolicy.AssignOnCreate)
			if tc.ownerCol != "" && tc.ownerCol != "id" {
				assert.NotContains(t, handlerData.ExcludeParamFields, "id")
				assert.Contains(t, handlerData.ExcludeParamFields, tc.ownerCol)
			}

			className := modelData.ClassName
			structContent := compileDemoStruct(className, tc.ownerCol, tc.ownerGo, tc.structOwner)
			modelData.Pk = "id"
			modelData.StructTemp = structContent

			modelCode, err := renderModel(modelData)
			if err != nil {
				raw, _ := renderRawModel(modelData)
				debug := "/tmp/crud-debug-raw-" + strings.ToLower(className)
				_ = os.MkdirAll(debug, 0755)
				_ = os.WriteFile(filepath.Join(debug, "model_raw.go"), []byte(raw), 0644)
				t.Logf("raw model code written to %s", debug)
			}
			require.NoError(t, err)

			handlerCode, err := renderHandler(handlerData)
			require.NoError(t, err)

			assert.NotContains(t, modelCode, "LimitAdminIds")
			assert.NotContains(t, modelCode, ".Save(&")
			assert.Contains(t, modelCode, "RowsAffected")
			if tc.cfg != nil && tc.cfg.Mode == data_scope.ModeNone {
				assert.Contains(t, modelCode, `Mode:            "none"`)
			}
			if tc.ownerCol != "" && tc.ownerCol != "id" && !(tc.cfg != nil && tc.cfg.Mode == data_scope.ModeNone) {
				assert.NotContains(t, handlerCode, `json:"`+tc.ownerCol+`"`)
				assert.NotContains(t, modelData.EditableColumns, tc.ownerCol)
			}
			if tc.cfg == nil || tc.cfg.Mode != data_scope.ModeNone {
				addIndex := strings.Index(modelCode, "func (s *"+className+"Repository) Add(")
				actorIndex := strings.Index(modelCode[addIndex:], "Enforcer.Actor(ctx)")
				transactionIndex := strings.Index(modelCode[addIndex:], "s.Transaction(ctx")
				assert.GreaterOrEqual(t, actorIndex, 0)
				assert.Greater(t, transactionIndex, actorIndex, "actor must be validated before Transaction")
			}

			if err := compileDataScopeFixture(t, className, modelCode, handlerCode, modelData.StructTemp, handlerData); err != nil {
				debug := "/tmp/crud-debug-" + strings.ToLower(className)
				_ = os.MkdirAll(debug, 0755)
				_ = os.WriteFile(filepath.Join(debug, "model.go"), []byte(modelCode), 0644)
				_ = os.WriteFile(filepath.Join(debug, "handler.go"), []byte(handlerCode), 0644)
				t.Logf("debug files written to %s", debug)
				t.Fatalf("compile fixture failed: %v", err)
			}
		})
	}
}

// TestPrepareGenerationData_Reassignable 验证 reassignable 时生成器不再剥离
// owner 列：表单保留、编辑列可写、DTO 携带 admin_id，且表单 markup 渲染为
// admin 的远程下拉（remote-url 反查真实 registrar 常量 /admin/auth.Admin/index）。
func TestPrepareGenerationData_Reassignable(t *testing.T) {
	cfg := &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true}
	table := crudmodel.Table{
		Name:           "seller_user",
		ModelFile:      "internal/admin/model/SellerUser.go",
		ControllerFile: "internal/admin/handler/SellerUser.go",
		WebViewsDir:    "web/src/views/backend/seller_user",
		FormFields:     []string{"name"},
		DataScope:      cfg,
	}
	fields := []crudmodel.Field{
		{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "admin_id", Type: "int", DesignType: "number"},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}
	getTableName := func(name string, full bool) string {
		if full {
			return "ba_" + name
		}
		return name
	}
	modelData, handlerData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, cfg, getTableName, proveAll)
	require.NoError(t, err)

	assert.True(t, modelData.DataScopePolicy.Reassignable)
	// ExcludeParamFields 不含 owner（DTO 保留 admin_id）
	assert.NotContains(t, handlerData.ExcludeParamFields, "admin_id")
	// EditableColumns 含 owner（编辑可写归属）
	assert.Contains(t, modelData.EditableColumns, "admin_id")
	// EffectiveFormFields 含 owner（未在 formFields 时强制 append）
	assert.Contains(t, modelData.EffectiveFormFields, "admin_id")
	// handler 数据携带 reassignable 与 owner Go 字段
	assert.True(t, handlerData.Reassignable)
	assert.Equal(t, "AdminID", handlerData.OwnerGoField)
	// 自动覆盖块注入 RelationFields（未手写时默认 username）：列表列关联显示
	// 列与后端关联加载器都以 RelationFields 为入口条件，缺失则列表缺上级代理
	// 列（下游 fork 反馈回归）。
	for i := range fields {
		if fields[i].Name == "admin_id" {
			assert.Equal(t, "remoteSelect", fields[i].DesignType)
			assert.Equal(t, "admin", fields[i].Form.RemoteTable)
			assert.Equal(t, "username", fields[i].Form.RelationFields)
		}
	}

	// DTO 保留 admin_id
	structContent := compileDemoStruct(modelData.ClassName, "admin_id", "AdminID", "admin_id")
	dtoCode, err := renderDTO(buildParamStruct(structContent, handlerData))
	require.NoError(t, err)
	assert.Contains(t, dtoCode, `json:"admin_id"`)

	// 表单 markup：owner 渲染为 remoteSelect，remote-url 反查 registrar 常量
	formMarkup := buildFormFieldMarkup(modelData.EffectiveFormFields, fields, "seller_user.", getTableName)
	joined := strings.Join(formMarkup, "\n")
	assert.Contains(t, joined, `prop="admin_id"`)
	assert.Contains(t, joined, `type="remoteSelect"`)
	assert.Contains(t, joined, `'remote-url': '/admin/auth.Admin/index`)
	assert.Contains(t, joined, `field: 'username'`)
}

// TestGeneratedReassignableCompiles 渲染 reassignable 的模型与 handler 并编译：
// Add 超管可指定归属、受限强制归属 + OwnerInScopeWithActor 校验；
// Edit 锁定当前行、归属变更时校验新 owner；handler 恢复未传(0)的归属。
func TestGeneratedReassignableCompiles(t *testing.T) {
	cfg := &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true}
	table := getTestTableData()
	table.DataScope = cfg
	fields := getCompileFields("admin_id")
	getTableName := func(name string, full bool) string {
		prefix := ""
		if full {
			prefix = "ba_"
		}
		return prefix + strings.TrimPrefix(name, prefix)
	}

	modelData, handlerData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, cfg, getTableName, proveAll)
	require.NoError(t, err)
	className := modelData.ClassName
	modelData.Pk = "id"
	modelData.StructTemp = compileDemoStruct(className, "admin_id", "AdminID", "admin_id")
	modelCode, err := renderModel(modelData)
	require.NoError(t, err)
	handlerCode, err := renderHandler(handlerData)
	require.NoError(t, err)

	assert.Contains(t, modelCode, `Reassignable:    true,`)
	assert.Contains(t, modelCode, `"gorm.io/gorm/clause"`)

	addIndex := strings.Index(modelCode, "func (s *"+className+"Repository) Add(")
	require.GreaterOrEqual(t, addIndex, 0)
	add := modelCode[addIndex:]
	assert.Contains(t, add, "if !(actor.Unrestricted && test1.AdminID > 0)")
	assert.Contains(t, add, "data_scope.OwnerInScopeWithActor(ctx, s.DBFor(ctx), s.Enforcer, s.config.Database.Prefix, int32(test1.AdminID), actor)")

	editIndex := strings.Index(modelCode, "func (s *"+className+"Repository) Edit(")
	require.GreaterOrEqual(t, editIndex, 0)
	edit := modelCode[editIndex:]
	assert.Contains(t, edit, `clause.Locking{Strength: "UPDATE"}`)
	assert.Contains(t, edit, "if current.AdminID != test1.AdminID {")
	assert.Contains(t, edit, "data_scope.OwnerInScopeWithActor(ctx, tx, s.Enforcer, s.config.Database.Prefix, int32(test1.AdminID), actor)")

	assert.Contains(t, handlerCode, "originalOwner := data.AdminID")
	assert.Contains(t, handlerCode, "if params.AdminID == 0 {")
	assert.Contains(t, handlerCode, "data.AdminID = originalOwner")

	if err := compileDataScopeFixture(t, className, modelCode, handlerCode, modelData.StructTemp, handlerData); err != nil {
		debug := "/tmp/crud-debug-reassignable"
		_ = os.MkdirAll(debug, 0755)
		_ = os.WriteFile(filepath.Join(debug, "model.go"), []byte(modelCode), 0644)
		_ = os.WriteFile(filepath.Join(debug, "handler.go"), []byte(handlerCode), 0644)
		t.Logf("debug files written to %s", debug)
		t.Fatalf("compile fixture failed: %v", err)
	}
}

// TestGeneratedReassignableBigIntOwnerRejected 覆盖 bigint owner 的生成期拒绝：
// 级联/重分配模板把 owner 以 int32(...) 传入 OwnerInScopeWithActor（admin id
// 域），bigint 列会"先截断校验、再写入未截断值"（fail-open 缺口），因此
// ResolveDataScope 直接拒绝（评审 MAJOR）。非 reassignable 的 bigint owner
// 不受影响（见 TestBigIntOwnerUsesInt64ActorConversion）。
func TestGeneratedReassignableBigIntOwnerRejected(t *testing.T) {
	cfg := &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true}
	table := getTestTableData()
	table.DataScope = cfg
	fields := getCompileFields("admin_id")
	for i := range fields {
		if fields[i].Name == "admin_id" {
			fields[i].Type = "bigint"
		}
	}
	getTableName := func(name string, full bool) string {
		prefix := ""
		if full {
			prefix = "ba_"
		}
		return prefix + strings.TrimPrefix(name, prefix)
	}

	_, _, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, cfg, getTableName, proveAll)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be int32-compatible")
	assert.Contains(t, err.Error(), "bigint owner columns are not supported")
}

// TestResolveDataScope_InheritFrom 覆盖 inheritFrom 的解析与校验矩阵：
// 与 reassignable 互斥、owner 列前置条件、标识符/自身表拒绝、byColumn 存在性、
// 合法组合通过。
func TestResolveDataScope_InheritFrom(t *testing.T) {
	fields := []crudmodel.Field{
		newField("id", "int", true),
		newField("admin_id", "int", false),
		newField("user_id", "int", false),
	}
	inherit := func(table, byColumn string) *data_scope.Config {
		return &data_scope.Config{Mode: data_scope.ModeAuto, InheritFrom: &data_scope.InheritRef{Table: table, ByColumn: byColumn}}
	}

	cases := []struct {
		name        string
		cfg         *data_scope.Config
		fields      []crudmodel.Field
		tableName   string
		wantErr     bool
		errContains string
		check       func(t *testing.T, resolved ResolvedDataScope)
	}{
		{
			name:        "inheritFrom mutually exclusive with reassignable",
			cfg:         &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true, InheritFrom: &data_scope.InheritRef{Table: "user", ByColumn: "user_id"}},
			fields:      fields,
			wantErr:     true,
			errContains: "mutually exclusive",
		},
		{
			name:        "inheritFrom by column missing from fields rejected",
			cfg:         inherit("user", "order_id"),
			fields:      fields,
			wantErr:     true,
			errContains: `inheritFrom by column "order_id" not found in table metadata`,
		},
		{
			name:        "inheritFrom requires owner column",
			cfg:         inherit("user", "user_id"),
			fields:      []crudmodel.Field{newField("id", "int", true), newField("user_id", "int", false)},
			wantErr:     true,
			errContains: "inheritFrom requires an owner column",
		},
		{
			name:        "inheritFrom self table rejected",
			cfg:         inherit("order", "user_id"),
			fields:      fields,
			tableName:   "order",
			wantErr:     true,
			errContains: `inheritFrom table "order" must not be the table itself`,
		},
		{
			name:        "inheritFrom invalid identifier rejected",
			cfg:         inherit("user.log", "user_id"),
			fields:      fields,
			wantErr:     true,
			errContains: "invalid inheritFrom table",
		},
		{
			name:   "inheritFrom valid with owner column",
			cfg:    inherit("user", "user_id"),
			fields: fields,
			check: func(t *testing.T, resolved ResolvedDataScope) {
				require.False(t, resolved.Policy.Reassignable)
				require.NotNil(t, resolved.Policy.InheritFrom)
				assert.Equal(t, &data_scope.InheritRef{Table: "user", ByColumn: "user_id"}, resolved.Policy.InheritFrom)
			},
		},
		{
			name:        "inheritFrom by column formBuildExclude rejected",
			cfg:         inherit("user", "user_id"),
			fields:      []crudmodel.Field{newField("id", "int", true), newField("admin_id", "int", false), {Name: "user_id", Type: "int", FormBuildExclude: true}},
			wantErr:     true,
			errContains: "must not be formBuildExclude'd",
		},
		{
			name:        "inheritFrom by column non-integer rejected",
			cfg:         inherit("user", "user_id"),
			fields:      []crudmodel.Field{newField("id", "int", true), newField("admin_id", "int", false), newField("user_id", "varchar", false)},
			wantErr:     true,
			errContains: "not integer-compatible",
		},
		{
			name:        "inheritFrom non-admin owner column rejected",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "agent_id", AssignOnCreate: ptr(true), InheritFrom: &data_scope.InheritRef{Table: "user", ByColumn: "user_id"}},
			fields:      []crudmodel.Field{newField("id", "int", true), newField("agent_id", "int", false), newField("user_id", "int", false)},
			wantErr:     true,
			errContains: "requires owner column admin_id",
		},
		{
			name:        "inheritFrom bigint owner rejected (int32 contract)",
			cfg:         inherit("user", "user_id"),
			fields:      []crudmodel.Field{newField("id", "int", true), newField("admin_id", "bigint", false), newField("user_id", "int", false)},
			wantErr:     true,
			errContains: "must be int32-compatible",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := ResolveDataScope(tc.cfg, tc.fields, DataScopeResolveOptions{ProveIndex: proveAll, TableName: tc.tableName})
			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				return
			}
			require.NoError(t, err)
			if tc.check != nil {
				tc.check(t, resolved)
			}
		})
	}
}

// TestModelTemplate_InheritFromAdd 渲染 inheritFrom 子表的 Add：归属继承自
// 主实体（FOR UPDATE + Select("admin_id") + Take(&inherited)），校验继承值在
// actor 麾下（校验实参硬编码 int32），无强制归属操作者/超管指定分支。
func TestModelTemplate_InheritFromAdd(t *testing.T) {
	data := ModelData{
		Namespace: "model", Name: "order", ClassName: "Order", ModelVar: "order",
		Pk: "id", PkGoField: "ID",
		DataScopePolicy:       data_scope.ResourcePolicy{Mode: data_scope.ModeAuto, OwnerColumn: "admin_id", AssignOnCreate: true},
		DataScopeOwnerGoField: "AdminID", DataScopeOwnerGoType: "int32",
		InheritFrom:       &data_scope.InheritRef{Table: "user", ByColumn: "user_id"},
		InheritByGoField:  "UserID",
		EditableColumns:   []string{"name"},
		EditableColumnsGo: `"name"`,
	}
	out := renderModelString(t, data)

	assert.Contains(t, out, `InheritFrom:     &data_scope.InheritRef{Table: "user", ByColumn: "user_id"}`)
	// Policy 字面量不再有 CascadeOwners 行（方案 A：注册表由生成器维护）。
	assert.NotContains(t, out, "CascadeOwners:")
	// 子表（inheritFrom）不生成 CascadeOwners() 注册方法。
	assert.NotContains(t, out, "func (s *OrderRepository) CascadeOwners()")
	assert.Contains(t, out, `"gorm.io/gorm/clause"`)

	addStart := strings.Index(out, "func (s *OrderRepository) Add(")
	require.GreaterOrEqual(t, addStart, 0)
	add := out[addStart:]
	// 继承分支：FOR UPDATE 锁定主实体行，读取其 admin_id。
	assert.Contains(t, add, `tx.Table(s.config.Database.Prefix+"user")`)
	assert.Contains(t, add, `clause.Locking{Strength: "UPDATE"}`)
	assert.Contains(t, add, `Select("admin_id")`)
	assert.Contains(t, add, `Where("id = ?", order.UserID)`)
	assert.Contains(t, add, "Take(&inherited)")
	// 赋值按列类型转换；校验实参硬编码 int32（bigint owner 教训）。
	assert.Contains(t, add, "order.AdminID = int32(inherited.AdminID)")
	assert.Contains(t, add, "data_scope.OwnerInScopeWithActor(ctx, tx, s.Enforcer, s.config.Database.Prefix, int32(order.AdminID), actor)")
	// 无强制归属操作者 / 超管指定分支。
	assert.NotContains(t, add, "actor.Unrestricted")
	assert.NotContains(t, add, "order.AdminID = int32(actor.AdminID)")
	// 继承分支在事务内完成（含时间戳与 create）。
	assert.Contains(t, add, "return s.Transaction(ctx, func(tx *gorm.DB) error {")
	assert.Contains(t, add, `tx.Table(s.TableName).Create(&order).Error`)
}

// TestModelTemplate_CascadeOwnersMethod 渲染 reassignable 主实体：生成
// CascadeOwners() 注册方法与锚点注释块；Edit 的级联改为运行时循环
// （s.CascadeOwners() + ValidateIdentifier + 反引号拼接），不再静态展开。
func TestModelTemplate_CascadeOwnersMethod(t *testing.T) {
	data := ModelData{
		Namespace: "model", Name: "seller_user", ClassName: "SellerUser", ModelVar: "sellerUser",
		Pk: "id", PkGoField: "ID",
		DataScopePolicy:       data_scope.ResourcePolicy{Mode: data_scope.ModeAuto, OwnerColumn: "admin_id", AssignOnCreate: true, Reassignable: true},
		DataScopeOwnerGoField: "AdminID", DataScopeOwnerGoType: "int32",
		EditableColumns:   []string{"name", "admin_id"},
		EditableColumnsGo: `"name", "admin_id"`,
	}
	out := renderModelString(t, data)

	// Policy 字面量：无 CascadeOwners 行，InheritFrom 保留。
	assert.NotContains(t, out, "CascadeOwners:")
	assert.Contains(t, out, "InheritFrom:     nil,")

	// 注册方法 + 锚点注释（独占一行）。
	methodStart := strings.Index(out, "func (s *SellerUserRepository) CascadeOwners() []data_scope.CascadeOwner {")
	require.GreaterOrEqual(t, methodStart, 0)
	method := out[methodStart:]
	assert.Contains(t, method, "// @cascade:begin")
	assert.Contains(t, method, "// @cascade:end")

	// Edit：运行时循环 + 标识符校验 + 反引号拼接；无静态展开。
	editStart := strings.Index(out, "func (s *SellerUserRepository) Edit(")
	require.GreaterOrEqual(t, editStart, 0)
	edit := out[editStart:]
	assert.Contains(t, edit, "for _, c := range s.CascadeOwners() {")
	assert.Contains(t, edit, "if err := data_scope.ValidateIdentifier(c.Table); err != nil {")
	assert.Contains(t, edit, "if err := data_scope.ValidateIdentifier(c.ByColumn); err != nil {")
	assert.Contains(t, edit, "if err := data_scope.ValidateIdentifier(c.OwnerColumn); err != nil {")
	assert.Contains(t, edit, `tx.Session(&gorm.Session{NewDB: true}).Table(s.config.Database.Prefix+c.Table)`)
	assert.Contains(t, edit, `.Where("`+"`"+`"+c.ByColumn+"`+"`"+` = ? AND `+"`"+`"+c.OwnerColumn+"`+"`"+` <> ?", sellerUser.ID, sellerUser.AdminID)`)
	assert.Contains(t, edit, `.Update(c.OwnerColumn, sellerUser.AdminID)`)
	assert.Contains(t, edit, `return fmt.Errorf("cascade owner %q: %w", c.Table, err)`)
	assert.NotContains(t, edit, `{{range .CascadeOwners}}`)
	// 级联循环在归属变更校验块内：先 OwnerInScopeWithActor 后级联。
	ownerCheck := strings.Index(edit, "data_scope.OwnerInScopeWithActor(ctx, tx, s.Enforcer, s.config.Database.Prefix, int32(sellerUser.AdminID), actor)")
	cascade := strings.Index(edit, "for _, c := range s.CascadeOwners() {")
	require.GreaterOrEqual(t, ownerCheck, 0)
	assert.Greater(t, cascade, ownerCheck)
}

// TestGeneratedInheritFromBigIntOwnerRejected 覆盖 bigint admin_id 子表 +
// inheritFrom 的生成期拒绝（评审 MAJOR：int32(...) 截断校验与实际写入值
// 不一致的 fail-open 缺口；非 int32 owner 在 ResolveDataScope 直接拒绝）。
func TestGeneratedInheritFromBigIntOwnerRejected(t *testing.T) {
	cfg := &data_scope.Config{Mode: data_scope.ModeAuto, InheritFrom: &data_scope.InheritRef{Table: "user", ByColumn: "user_id"}}
	table := getTestTableData()
	table.Name = "order"
	table.ModelFile = "internal/admin/model/Order.go"
	table.ControllerFile = "internal/admin/handler/Order.go"
	table.DataScope = cfg
	fields := getCompileFields("admin_id")
	fields = append(fields, crudmodel.Field{Name: "user_id", Type: "int", DesignType: "int"})
	for i := range fields {
		if fields[i].Name == "admin_id" {
			fields[i].Type = "bigint"
		}
	}
	getTableName := func(name string, full bool) string {
		prefix := ""
		if full {
			prefix = "ba_"
		}
		return prefix + strings.TrimPrefix(name, prefix)
	}

	_, _, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, cfg, getTableName, proveAll)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be int32-compatible")
	assert.Contains(t, err.Error(), "bigint owner columns are not supported")
}

// TestGeneratedCascadeRegistryCompiles 覆盖主表 + 注册方法的编译：把生成器会
// 注入的锚点条目（applyCascadeAnchor 格式）写入渲染结果的锚点块后整体编译，
// Edit 级联循环使用列类型值（int32）不引入转换问题。
func TestGeneratedCascadeRegistryCompiles(t *testing.T) {
	cfg := &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true}
	table := getTestTableData()
	table.Name = "seller_user"
	table.ModelFile = "internal/admin/model/SellerUser.go"
	table.ControllerFile = "internal/admin/handler/SellerUser.go"
	table.DataScope = cfg
	fields := getCompileFields("admin_id")
	getTableName := func(name string, full bool) string {
		prefix := ""
		if full {
			prefix = "ba_"
		}
		return prefix + strings.TrimPrefix(name, prefix)
	}

	modelData, handlerData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, cfg, getTableName, proveAll)
	require.NoError(t, err)
	assert.Equal(t, "int32", modelData.DataScopeOwnerGoType)
	className := modelData.ClassName
	modelData.Pk = "id"
	modelData.StructTemp = compileDemoStruct(className, "admin_id", "AdminID", "admin_id")
	modelCode, err := renderModel(modelData)
	require.NoError(t, err)
	handlerCode, err := renderHandler(handlerData)
	require.NoError(t, err)

	// 模拟生成器维护：向锚点块注入条目（applyCascadeAnchor 的产物格式）。
	modelCode = injectCascadeAnchorEntriesForTest(t, modelCode, "user_order")

	editIndex := strings.Index(modelCode, "func (s *"+className+"Repository) Edit(")
	require.GreaterOrEqual(t, editIndex, 0)
	edit := modelCode[editIndex:]
	assert.Contains(t, edit, "for _, c := range s.CascadeOwners() {")
	assert.Contains(t, edit, `s.config.Database.Prefix+c.Table`)

	if err := compileDataScopeFixture(t, className, modelCode, handlerCode, modelData.StructTemp, handlerData); err != nil {
		debug := "/tmp/crud-debug-cascade"
		_ = os.MkdirAll(debug, 0755)
		_ = os.WriteFile(filepath.Join(debug, "model.go"), []byte(modelCode), 0644)
		_ = os.WriteFile(filepath.Join(debug, "handler.go"), []byte(handlerCode), 0644)
		t.Logf("debug files written to %s", debug)
		t.Fatalf("compile fixture failed: %v", err)
	}
}

// injectCascadeAnchorEntriesForTest 把渲染结果中的空锚点占位替换为注册条目行
// （与 cascadeAnchorEntry 同格式，模拟生成器的 applyCascadeAnchor 产物）。
func injectCascadeAnchorEntriesForTest(t *testing.T, repoCode string, tables ...string) string {
	t.Helper()
	placeholder := "\t\t// @cascade:begin\n\t\t// @cascade:end"
	if !strings.Contains(repoCode, placeholder) {
		t.Fatalf("anchor placeholder not found in rendered repo:\n%s", repoCode)
	}
	var entries strings.Builder
	for _, table := range tables {
		entries.WriteString("\n\t\t" + cascadeAnchorEntry(table, "user_id"))
	}
	return strings.Replace(repoCode, placeholder, "\t\t// @cascade:begin"+entries.String()+"\n\t\t// @cascade:end", 1)
}

func renderModelString(t *testing.T, data ModelData) string {
	t.Helper()
	if data.StructTemp == "" {
		data.StructTemp = compileDemoStruct(data.ClassName, "admin_id", "AdminID", "admin_id")
	}
	out, err := renderModel(data)
	require.NoError(t, err)
	return out
}

func getCompileFields(owner string) []crudmodel.Field {
	fields := []crudmodel.Field{
		{Name: "id", Type: "int", DesignType: "int", PrimaryKey: true, FormBuildExclude: true},
		{Name: "name", Type: "varchar", DesignType: "varchar"},
		{Name: "create_time", Type: "int", DesignType: "int", FormBuildExclude: true},
		{Name: "update_time", Type: "int", DesignType: "int", FormBuildExclude: true},
	}
	if owner != "" && owner != "id" {
		fields = append(fields, crudmodel.Field{Name: owner, Type: "int", DesignType: "int"})
	}
	if owner == "" {
		fields = append(fields, crudmodel.Field{Name: "admin_id", Type: "int", DesignType: "int"})
	}
	return fields
}

func compileDemoStruct(className, ownerCol, ownerGo, structOwner string) string {
	var b strings.Builder
	b.WriteString("import (\n")
	b.WriteString("\t\"github.com/gin-gonic/gin\"\n")
	b.WriteString("\t\"gorm.io/gorm\"\n")
	b.WriteString("\t\"buildadmin-go/internal/conf\"\n")
	b.WriteString(")\n")
	b.WriteString("// " + className + " demo table\n")
	b.WriteString("type " + className + " struct {\n")
	b.WriteString("\tID int32 `gorm:\"column:id;primaryKey;autoIncrement:true\" json:\"id\"`\n")
	if structOwner == "admin_id" {
		b.WriteString("\tAdminID int32 `gorm:\"column:admin_id\" json:\"admin_id\"`\n")
	}
	if ownerCol != "" && ownerCol != "id" && ownerCol != "admin_id" {
		b.WriteString("\t" + ownerGo + " int32 `gorm:\"column:" + ownerCol + "\" json:\"" + ownerCol + "\"`\n")
	}
	b.WriteString("\tName string `gorm:\"column:name\" json:\"name\"`\n")
	b.WriteString("\tCreateTime int64 `gorm:\"column:create_time\" json:\"create_time\"`\n")
	b.WriteString("\tUpdateTime int64 `gorm:\"column:update_time\" json:\"update_time\"`\n")
	b.WriteString("}\n")
	return b.String()
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("cannot find repo root (go.mod)")
		}
		dir = parent
	}
}

func compileDataScopeFixture(t *testing.T, className, modelCode, handlerCode, structContent string, handlerData HandlerData) error {
	t.Helper()
	root := repoRoot(t)
	tmp := t.TempDir()

	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	goSum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), goMod, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.sum"), goSum, 0644); err != nil {
		return err
	}

	srcDataScope := filepath.Join(root, "internal", "pkg", "data_scope")
	dstDataScope := filepath.Join(tmp, "internal", "pkg", "data_scope")
	if err := copyDir(srcDataScope, dstDataScope); err != nil {
		return err
	}

	stubs := map[string]string{
		"internal/conf/config.go": `package conf

type Configuration struct {
	Database Database
}

type Database struct {
	Prefix string
}
`,
		"internal/pkg/persistence/base.go": `package persistence

import (
	"context"

	"gorm.io/gorm"
)

type TableInfo struct {
	TableName        string
	Key              string
	QuickSearchField string
}

type BaseModel struct {
	TableName        string
	Key              string
	QuickSearchField string
	sqlDB            *gorm.DB
}

func NewBaseModel(tableName, key, quickSearchField string, sqlDB *gorm.DB) BaseModel {
	return BaseModel{TableName: tableName, Key: key, QuickSearchField: quickSearchField, sqlDB: sqlDB}
}

func (b *BaseModel) DBFor(context.Context) *gorm.DB { return b.sqlDB }
func (b *BaseModel) Transaction(_ context.Context, fn func(*gorm.DB) error) error { return b.sqlDB.Transaction(fn) }
func (b *BaseModel) TableInfo() TableInfo {
	return TableInfo{TableName: b.TableName, Key: b.Key, QuickSearchField: b.QuickSearchField}
}
`,
		"internal/admin/repository/query_builder.go": `package repository

import (
	"buildadmin-go/internal/pkg/persistence"
	"github.com/gin-gonic/gin"
)

type TableInfo = persistence.TableInfo

func QueryBuilder(ctx *gin.Context, table TableInfo, where []TableInfo) (string, []interface{}, string, int, int, error) {
	return "", nil, "", 0, 0, nil
}
`,
		"internal/admin/repository/provider.go": `package repository

import "github.com/google/wire"

var ProviderSet = wire.NewSet()
`,
		"internal/admin/handler/base.go": `package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Base struct {
	currentM any
	log      *zap.Logger
}

func (b *Base) Select(ctx *gin.Context) (any, bool) { return nil, false }
func (b *Base) MaybePartialEdit(ctx *gin.Context, fields map[string]bool) bool { return false }
`,
		"internal/pkg/response/response.go": `package response

import (
	"github.com/gin-gonic/gin"
)

type Response struct {
	Code int
	Data any
	Msg  string
	Time int64
}

func Success(c *gin.Context, data any)                  {}
func SuccessWithMessage(c *gin.Context, message string) {}
func FailByErr(c *gin.Context, err error)               {}
`,
		"internal/pkg/validator/validator.go": `package validator

type FlexInt32 int32
type FlexInt64 int64
type FlexFloat64 float64

type Ids struct {
	Ids interface{}
}

func GetError(v interface{}, err error) error { return err }
`,
	}
	for name, body := range stubs {
		path := filepath.Join(tmp, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			return err
		}
	}

	entityCode, err := renderEntity(ModelData{
		Namespace:  "model",
		ClassName:  className,
		StructTemp: structContent,
		ModelVar:   lowerFirst(className),
		PkGoType:   "int32",
		PkGoField:  "ID",
		Pk:         "id",
	})
	if err != nil {
		return err
	}
	entityDir := filepath.Join(tmp, "internal", "model")
	if err := os.MkdirAll(entityDir, 0755); err != nil {
		return err
	}
	entityPath := filepath.Join(entityDir, strings.ToLower(className)+".go")
	if err := os.WriteFile(entityPath, []byte(entityCode), 0644); err != nil {
		return err
	}
	modelPath := filepath.Join(tmp, "internal", "admin", "repository", strings.ToLower(className)+"_gen.go")
	if err := os.WriteFile(modelPath, []byte(modelCode), 0644); err != nil {
		return err
	}
	handlerPath := filepath.Join(tmp, "internal", "admin", "handler", strings.ToLower(className)+"_handler.go")
	if err := os.WriteFile(handlerPath, []byte(handlerCode), 0644); err != nil {
		return err
	}
	dtoDir := filepath.Join(tmp, "internal", "admin", "dto")
	if err := os.MkdirAll(dtoDir, 0755); err != nil {
		return err
	}
	dtoPath := filepath.Join(dtoDir, strings.ToLower(className)+".go")
	dtoParam := "type " + className + "Param struct {\n\tName string `json:\"name\"`\n}\n"
	if handlerData.ClassName != "" {
		if built := buildParamStruct(structContent, handlerData); built != "" {
			dtoParam = built
		}
	}
	dtoCode, err := renderDTO(dtoParam)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dtoPath, []byte(dtoCode), 0644); err != nil {
		return err
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = tmp
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("go build output:\n%s", out)
		return err
	}
	return nil
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, name), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func renderRawModel(data ModelData) (string, error) {
	tpl, err := template.New("model").Parse(modelTemp)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
