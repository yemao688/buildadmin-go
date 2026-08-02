package crud_helper

import (
	"buildadmin-go/internal/utils"
	"path/filepath"
	"testing"
)

func TestCountrySpecsMatchDictionaryContract(t *testing.T) {
	cases := []struct {
		file        string
		field       string
		type_       string
		length      int
		unsigned    bool
		defaultType string
		comment     string
	}{
		{"country_currency.yaml", "id", "bigint", 0, true, "NONE", "ID"},
		{"country_currency.yaml", "code", "varchar", 20, false, "EMPTY STRING", "货币代码"},
		{"country_currency.yaml", "status", "tinyint", 0, true, "INPUT", "状态:0=禁用,1=启用"},
		{"country_language.yaml", "remark", "varchar", 255, false, "EMPTY STRING", "备注"},
		{"country_language_content.yaml", "type", "varchar", 30, false, "EMPTY STRING", "类型:0=文本,1=富文本,2=图片"},
		{"country_language_content.yaml", "value", "longtext", 0, false, "NONE", "值"},
	}

	for _, test := range cases {
		t.Run(test.file+"/"+test.field, func(t *testing.T) {
			spec, err := LoadSpec(filepath.Join(utils.RootPath(), "crud_specs", test.file))
			if err != nil {
				t.Fatal(err)
			}
			var fieldName string
			for _, field := range spec.Fields {
				if field.Name == test.field {
					fieldName = field.Name
					if field.Type != test.type_ || field.Length != test.length || field.Unsigned != test.unsigned || field.DefaultType != test.defaultType || field.Comment != test.comment {
						t.Fatalf("field = %+v", field)
					}
					break
				}
			}
			if fieldName == "" {
				t.Fatalf("field %q not found", test.field)
			}
		})
	}
}
