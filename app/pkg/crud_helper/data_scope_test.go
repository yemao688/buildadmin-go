package crud_helper

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
)

func ptr[T any](v T) *T { return &v }

func newField(name, typ string, pk bool) model.Field {
	return model.Field{Name: name, Type: typ, DesignType: typ, PrimaryKey: pk}
}

func proveAll(_ string) (bool, error)  { return true, nil }
func proveNone(_ string) (bool, error) { return false, nil }

func TestResolveDataScope_Matrix(t *testing.T) {
	cases := []struct {
		name        string
		cfg         *data_scope.Config
		fields      []model.Field
		allowNone   bool
		prove       func(string) (bool, error)
		wantMode    data_scope.Mode
		wantOwner   string
		wantGoField string
		wantAssign  bool
		wantIndex   IndexStrategy
		wantErr     bool
		errContains string
	}{
		{
			name:        "legacy nil with exact admin_id fails closed without index proof",
			cfg:         nil,
			fields:      []model.Field{newField("id", "int", true), newField("admin_id", "int", false)},
			prove:       proveNone,
			wantErr:     true,
			errContains: "no proven index",
		},
		{
			name:        "legacy nil with exact admin_id and proven index",
			cfg:         nil,
			fields:      []model.Field{newField("id", "int", true), newField("admin_id", "int", false)},
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
			fields:   []model.Field{newField("id", "int", true), newField("AdminID", "int", false)},
			prove:    proveAll,
			wantMode: data_scope.ModeNone,
		},
		{
			name:     "auto with last_admin_id is not auto owner",
			cfg:      &data_scope.Config{Mode: data_scope.ModeAuto},
			fields:   []model.Field{newField("id", "int", true), newField("last_admin_id", "int", false)},
			prove:    proveAll,
			wantMode: data_scope.ModeNone,
		},
		{
			name:        "required custom owner last_admin_id without proven index fails",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "last_admin_id"},
			fields:      []model.Field{newField("id", "int", true), newField("last_admin_id", "int", false)},
			prove:       proveNone,
			wantErr:     true,
			errContains: "no proven index",
		},
		{
			name:        "required custom owner operator_admin_id with proven index",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "operator_admin_id"},
			fields:      []model.Field{newField("id", "int", true), newField("operator_admin_id", "int", false)},
			prove:       proveAll,
			wantMode:    data_scope.ModeRequired,
			wantOwner:   "operator_admin_id",
			wantGoField: "OperatorAdminID",
			wantIndex:   IndexProven,
		},
		{
			name:        "required missing owner column",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired},
			fields:      []model.Field{newField("id", "int", true)},
			wantErr:     true,
			errContains: "owner column is required",
		},
		{
			name:        "required owner column wrong type",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "owner_name"},
			fields:      []model.Field{newField("id", "int", true), newField("owner_name", "varchar", false)},
			wantErr:     true,
			errContains: "not integer-compatible",
		},
		{
			name:        "required custom owner with assign on create",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "owner_id", AssignOnCreate: ptr(true)},
			fields:      []model.Field{newField("id", "int", true), newField("owner_id", "int", false)},
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
			fields:      []model.Field{newField("id", "int", true), newField("admin_id", "int", false)},
			prove:       proveAll,
			wantErr:     true,
			errContains: "explicit override",
		},
		{
			name:      "none override with admin_id allowed when explicit",
			cfg:       &data_scope.Config{Mode: data_scope.ModeNone},
			fields:    []model.Field{newField("id", "int", true), newField("admin_id", "int", false)},
			allowNone: true,
			prove:     proveAll,
			wantMode:  data_scope.ModeNone,
		},
		{
			name:        "admin.id explicit required",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "id"},
			fields:      []model.Field{newField("id", "int", true)},
			wantMode:    data_scope.ModeRequired,
			wantOwner:   "id",
			wantGoField: "ID",
			wantAssign:  false,
			wantIndex:   IndexProven,
		},
		{
			name:        "admin.id cannot assign on create",
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "id", AssignOnCreate: ptr(true)},
			fields:      []model.Field{newField("id", "int", true)},
			wantErr:     true,
			errContains: "admin.id cannot assign on create",
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
			assert.Equal(t, tc.wantIndex, got.IndexStrategy)
		})
	}
}

func TestResolveDataScope_NoProverFailsClosed(t *testing.T) {
	fields := []model.Field{newField("id", "int", true), newField("admin_id", "int", false)}
	_, err := ResolveDataScope(nil, fields, DataScopeResolveOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot prove an index")
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
	assert.Contains(t, out, "func (s *DemoModel) scopedDB")
	assert.Contains(t, out, "func (s *DemoModel) ScopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB")
	assert.Contains(t, out, "data_scope.OwnerRef{TableAlias: s.TableName, Column: s.Policy.OwnerColumn}")
	assert.Contains(t, out, "demo.AdminID = actor.AdminID")
	assert.Contains(t, out, `Where("id = ?", demo.ID)`)
	assert.Contains(t, out, "RowsAffected")
	assert.NotContains(t, out, "LimitAdminIds")
	assert.NotContains(t, out, ".Save(&demo)")
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
	assert.Contains(t, out, `Mode:           "none"`)
	assert.Contains(t, out, "return s.scopeDB(ctx, s.DBFor(ctx))")
	assert.Contains(t, out, "return s.scopeDB(ctx, db)")
	assert.NotContains(t, out, "actor.AdminID")
	assert.NotContains(t, out, "LimitAdminIds")
	assert.NotContains(t, out, ".Save(&demo)")
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
	table := model.Table{
		Name:       "demo",
		DataScope:  cfg,
		FormFields: []string{"name", "operator_admin_id"},
	}

	data, err := json.Marshal(table)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"dataScope":`)
	assert.Contains(t, string(data), `"mode":"required"`)
	assert.Contains(t, string(data), `"ownerColumn":"operator_admin_id"`)

	var decoded model.Table
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.NotNil(t, decoded.DataScope)
	assert.Equal(t, data_scope.ModeRequired, decoded.DataScope.Mode)
	assert.Equal(t, "operator_admin_id", decoded.DataScope.OwnerColumn)

	// Legacy nil DataScope resolves to auto when fed through generation.
	legacy := model.Table{Name: "legacy"}
	legacyData, err := json.Marshal(legacy)
	require.NoError(t, err)
	var legacyDecoded model.Table
	require.NoError(t, json.Unmarshal(legacyData, &legacyDecoded))
	assert.Nil(t, legacyDecoded.DataScope)
}

func TestEffectiveFormFieldsReachPopupFormRender(t *testing.T) {
	cases := []struct {
		name       string
		cfg        *data_scope.Config
		owner      string
		wantOwner  bool
		ownerField model.Field
	}{
		{
			name:  "auto admin_id",
			owner: "admin_id",
			ownerField: model.Field{
				Name: "admin_id", Type: "int", DesignType: "number",
			},
		},
		{
			name:  "custom owner",
			cfg:   &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "operator_admin_id"},
			owner: "operator_admin_id",
			ownerField: model.Field{
				Name: "operator_admin_id", Type: "int", DesignType: "number",
			},
		},
		{
			name:  "admin.id",
			cfg:   &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "id"},
			owner: "id",
			ownerField: model.Field{
				Name: "id", Type: "int", DesignType: "number", PrimaryKey: true,
			},
		},
		{
			name:       "explicit global retains admin_id",
			cfg:        &data_scope.Config{Mode: data_scope.ModeNone},
			owner:      "admin_id",
			wantOwner:  true,
			ownerField: model.Field{Name: "admin_id", Type: "int", DesignType: "number"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table := model.Table{
				Name:           "form_scope_test",
				Comment:        "form scope",
				ModelFile:      "app/admin/model/FormScopeTest.go",
				ControllerFile: "app/admin/handler/FormScopeTest.go",
				WebViewsDir:    "web/src/views/backend/form_scope_test",
				FormFields:     []string{"name", tc.owner},
				DataScope:      tc.cfg,
			}
			fields := []model.Field{
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
			modelData, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, tc.cfg, getTableName, proveAll)
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
			cfg:         &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "operator_admin_id"},
			ownerCol:    "operator_admin_id",
			ownerGo:     "OperatorAdminID",
			assign:      false,
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

			modelData, handlerData, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, tc.cfg, getTableName, proveAll)
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

			handlerCode, err := renderHandler(handlerData, structContent)
			require.NoError(t, err)

			assert.NotContains(t, modelCode, "LimitAdminIds")
			assert.NotContains(t, modelCode, ".Save(&")
			assert.Contains(t, modelCode, "RowsAffected")
			if tc.cfg != nil && tc.cfg.Mode == data_scope.ModeNone {
				assert.Contains(t, modelCode, `Mode:           "none"`)
			}
			if tc.ownerCol != "" && tc.ownerCol != "id" && !(tc.cfg != nil && tc.cfg.Mode == data_scope.ModeNone) {
				assert.NotContains(t, handlerCode, `json:"`+tc.ownerCol+`"`)
				assert.NotContains(t, modelData.EditableColumns, tc.ownerCol)
			}
			if tc.cfg == nil || tc.cfg.Mode != data_scope.ModeNone {
				addIndex := strings.Index(modelCode, "func (s *"+className+"Model) Add(")
				actorIndex := strings.Index(modelCode[addIndex:], "Enforcer.Actor(ctx)")
				transactionIndex := strings.Index(modelCode[addIndex:], "s.Transaction(ctx")
				assert.GreaterOrEqual(t, actorIndex, 0)
				assert.Greater(t, transactionIndex, actorIndex, "actor must be validated before Transaction")
			}

			if err := compileDataScopeFixture(t, className, modelCode, handlerCode); err != nil {
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

func renderModelString(t *testing.T, data ModelData) string {
	t.Helper()
	if data.StructTemp == "" {
		data.StructTemp = compileDemoStruct(data.ClassName, "admin_id", "AdminID", "admin_id")
	}
	out, err := renderModel(data)
	require.NoError(t, err)
	return out
}

func getCompileFields(owner string) []model.Field {
	fields := []model.Field{
		{Name: "id", Type: "int", DesignType: "int", PrimaryKey: true, FormBuildExclude: true},
		{Name: "name", Type: "varchar", DesignType: "varchar"},
		{Name: "create_time", Type: "int", DesignType: "int", FormBuildExclude: true},
		{Name: "update_time", Type: "int", DesignType: "int", FormBuildExclude: true},
	}
	if owner != "" && owner != "id" {
		fields = append(fields, model.Field{Name: owner, Type: "int", DesignType: "int"})
	}
	if owner == "" {
		fields = append(fields, model.Field{Name: "admin_id", Type: "int", DesignType: "int"})
	}
	return fields
}

func compileDemoStruct(className, ownerCol, ownerGo, structOwner string) string {
	var b strings.Builder
	b.WriteString("import (\n")
	b.WriteString("\t\"github.com/gin-gonic/gin\"\n")
	b.WriteString("\t\"gorm.io/gorm\"\n")
	b.WriteString("\t\"go-build-admin/conf\"\n")
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

func compileDataScopeFixture(t *testing.T, className, modelCode, handlerCode string) error {
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

	srcDataScope := filepath.Join(root, "app", "pkg", "data_scope")
	dstDataScope := filepath.Join(tmp, "app", "pkg", "data_scope")
	if err := copyDir(srcDataScope, dstDataScope); err != nil {
		return err
	}

	stubs := map[string]string{
		"conf/config.go": `package conf

type Configuration struct {
	Database Database
}

type Database struct {
	Prefix string
}
`,
		"app/admin/model/base.go": `package model

import (
	"context"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BaseModel struct {
	TableName        string
	Key              string
	QuickSearchField string
	sqlDB            *gorm.DB
}

func (b *BaseModel) TableInfo() map[string]string { return map[string]string{} }
func (b *BaseModel) DBFor(context.Context) *gorm.DB { return b.sqlDB }
func (b *BaseModel) Transaction(_ context.Context, fn func(*gorm.DB) error) error { return b.sqlDB.Transaction(fn) }

func QueryBuilder(ctx *gin.Context, tableInfo map[string]string, where map[string]interface{}) (string, []interface{}, string, int, int, error) {
	return "", nil, "", 0, 0, nil
}
`,
		"app/admin/handler/base.go": `package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Base struct {
	currentM any
	log      *zap.Logger
}

type IDS struct {
	ID int32
}

func (b *Base) Select(ctx *gin.Context) (any, bool) { return nil, false }
func (b *Base) MaybePartialEdit(ctx *gin.Context, fields map[string]bool) bool { return false }
func Success(ctx *gin.Context, data any)            {}
func FailByErr(ctx *gin.Context, err error)         {}
`,
		"app/admin/validate/validate.go": `package validate

type Ids struct {
	Ids interface{}
}

func GetError(v interface{}, err error) error { return err }
`,
		"app/pkg/validator/validator.go": `package validator

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

	modelPath := filepath.Join(tmp, "app", "admin", "model", strings.ToLower(className)+"_gen.go")
	if err := os.WriteFile(modelPath, []byte(modelCode), 0644); err != nil {
		return err
	}
	handlerPath := filepath.Join(tmp, "app", "admin", "handler", strings.ToLower(className)+"_handler.go")
	if err := os.WriteFile(handlerPath, []byte(handlerCode), 0644); err != nil {
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
