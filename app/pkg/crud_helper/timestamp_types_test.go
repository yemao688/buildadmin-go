package crud_helper

import (
	"strings"
	"testing"

	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
)

func TestTimestampTypeOverridesSkipCanonicalAndSplitOthers(t *testing.T) {
	fields := []model.Field{
		{Name: "create_time", Type: "bigint", DataType: "bigint", DesignType: "timestamp"},
		{Name: "createtime", Type: "bigint", DataType: "bigint", DesignType: "timestamp"},
		{Name: "update_time", Type: "bigint", DataType: "bigint", DesignType: "timestamp"},
		{Name: "updatetime", Type: "bigint", DataType: "bigint", DesignType: "timestamp"},
		{Name: "end_time", Type: "bigint", DataType: "bigint", DesignType: "timestamp"},
		{Name: "created_at", Type: "datetime", DataType: "datetime", DesignType: "datetime"},
	}
	analysed := make([]model.Field, len(fields))
	for i, field := range fields {
		analysed[i] = analyseField(field)
	}
	modelTypes := buildModelFieldTypeOverrides(fields)
	paramTypes := buildHandlerParamTypeOverrides(analysed)
	for _, name := range []string{"create_time", "createtime", "update_time", "updatetime"} {
		if _, ok := modelTypes[name]; ok {
			t.Fatalf("canonical model override %s should be skipped, got %q", name, modelTypes[name])
		}
		if _, ok := paramTypes[name]; ok {
			t.Fatalf("canonical handler override %s should be skipped, got %q", name, paramTypes[name])
		}
	}
	if modelTypes["end_time"] != "validate.FlexFormattedUnixTime" || paramTypes["end_time"] != "validate.FlexFormattedUnixTime" {
		t.Fatalf("end_time overrides = model %q handler %q", modelTypes["end_time"], paramTypes["end_time"])
	}
	if modelTypes["created_at"] != "validate.FlexDateTime" || paramTypes["created_at"] != "validate.FlexDateTime" {
		t.Fatalf("created_at overrides = model %q handler %q", modelTypes["created_at"], paramTypes["created_at"])
	}
}

func TestPrepareModelTimestampDataDetectsJSONTagsAndAssignments(t *testing.T) {
	data := ModelData{
		StructTemp: "type Demo struct {\n" +
			"\tCreateTime int64 `json:\"create_time\"`\n" +
			"\tUpdateTime int64 `json:\"update_time\"`\n" +
			"}\n",
	}
	prepareModelTimestampData(&data)
	if !data.HasCreateTime || data.CreateTime != "CreateTime" {
		t.Fatalf("create_time detection = has %v field %q", data.HasCreateTime, data.CreateTime)
	}
	if !data.HasUpdateTime || data.UpdateTime != "UpdateTime" {
		t.Fatalf("update_time detection = has %v field %q", data.HasUpdateTime, data.UpdateTime)
	}

	data = ModelData{
		StructTemp: "type Demo struct {\n" +
			"\tCreatetime int64 `json:\"createtime\"`\n" +
			"\tUpdatetime int64 `json:\"updatetime\"`\n" +
			"}\n",
	}
	prepareModelTimestampData(&data)
	if !data.HasCreateTime || data.CreateTime != "Createtime" {
		t.Fatalf("createtime detection = has %v field %q", data.HasCreateTime, data.CreateTime)
	}
	if !data.HasUpdateTime || data.UpdateTime != "Updatetime" {
		t.Fatalf("updatetime detection = has %v field %q", data.HasUpdateTime, data.UpdateTime)
	}
}

func TestRenderModelUsesOneNowAndActualCanonicalFieldNames(t *testing.T) {
	modelData := ModelData{
		Namespace:        "model",
		ClassName:        "Demo",
		ModelVar:         "demo",
		Pk:               "id",
		PkGoType:         "int64",
		PkGoField:        "ID",
		QuickSearchField: "id",
		StructTemp: "type Demo struct {\n" +
			"\tID int64 `json:\"id\"`\n" +
			"\tCreatetime int64 `json:\"createtime\"`\n" +
			"\tUpdatetime int64 `json:\"updatetime\"`\n" +
			"}\n",
		DataScopePolicy:   data_scope.ResourcePolicy{Mode: data_scope.ModeNone},
		EditableColumns:   []string{"updatetime"},
		EditableColumnsGo: joinQuotedColumns([]string{"updatetime"}),
	}
	code, err := renderModel(modelData)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(code, "time.Now().Unix()") != 2 {
		t.Fatalf("expected one Add now variable plus one Edit update assignment, code:\n%s", code)
	}
	if !strings.Contains(code, "now := time.Now().Unix()") {
		t.Fatalf("Add now assignment missing:\n%s", code)
	}
	if !strings.Contains(code, "demo.Createtime = now") || !strings.Contains(code, "demo.Updatetime = now") {
		t.Fatalf("Add canonical assignments missing:\n%s", code)
	}
	if !strings.Contains(code, "demo.Updatetime = time.Now().Unix()") {
		t.Fatalf("Edit update assignment missing:\n%s", code)
	}
	if strings.Contains(code, "demo.CreateTime =") || strings.Contains(code, "demo.UpdateTime =") {
		t.Fatalf("hardcoded camel-case names should not appear:\n%s", code)
	}
}

func TestCanonicalTimestampsExcludedFromEditableColumnsAndParams(t *testing.T) {
	fields := []model.Field{
		{Name: "id", PrimaryKey: true, FormBuildExclude: true},
		{Name: "create_time", DesignType: "timestamp", FormBuildExclude: true},
		{Name: "update_time", DesignType: "timestamp", FormBuildExclude: true},
		{Name: "title", DesignType: "string"},
	}
	editable := buildEditableColumns("id", "", []string{"create_time", "update_time", "title"}, fields)
	if len(editable) != 1 || editable[0] != "title" {
		t.Fatalf("editable columns = %#v", editable)
	}
	input := "type DemoParam struct {\n" +
		"\tCreateTime int64 `json:\"create_time\"`\n" +
		"\tUpdateTime int64 `json:\"update_time\"`\n" +
		"\tTitle string `json:\"title\"`\n" +
		"}\n"
	out := excludeParamFields(input, []string{"create_time", "update_time"})
	if strings.Contains(out, "create_time") || strings.Contains(out, "update_time") {
		t.Fatalf("canonical timestamp params not excluded:\n%s", out)
	}
	if !strings.Contains(out, "title") {
		t.Fatalf("non-canonical field removed unexpectedly:\n%s", out)
	}
}
