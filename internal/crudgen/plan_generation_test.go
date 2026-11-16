package crudgen

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"strings"
	"testing"
)

func TestPreviewGenerationFilesSeparatesCreateAndOverwrite(t *testing.T) {
	t.Parallel()
	fields := []crudmodel.Field{{Name: "id", Type: "bigint", Unsigned: true, PrimaryKey: true, AutoIncrement: true, DesignType: "pk"}}
	plan, err := PreviewGenerationFiles(nil, nil, crudmodel.Table{Name: "ops_preview_demo"}, fields)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Create) == 0 || len(plan.Overwrite) == 0 {
		t.Fatalf("plan = %+v", plan)
	}
	var createdEntity bool
	for _, path := range plan.Create {
		if strings.HasSuffix(path, "internal/model/ops_preview_demo.go") {
			createdEntity = true
		}
	}
	if !createdEntity {
		t.Fatalf("entity file must be planned as new: %+v", plan.Create)
	}
	var sharedProvider bool
	for _, path := range plan.Overwrite {
		if strings.HasSuffix(path, "internal/admin/router/provider.go") {
			sharedProvider = true
		}
	}
	if !sharedProvider {
		t.Fatalf("router provider must be planned as overwrite: %+v", plan.Overwrite)
	}
}
