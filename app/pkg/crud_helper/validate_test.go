package crud_helper

import (
	"strings"
	"testing"
)

func TestValidateSpecAcceptsValidRelationsAndFiles(t *testing.T) {
	path := writeSpecTest(t, `name: orders
fields:
  - name: id
    type: bigint
    primaryKey: true
  - name: name
    type: varchar
  - name: owner_id
    type: bigint
    designType: remoteSelect
    form:
      remoteTable: admin
      remotePk: id
      remoteField: username
      relationFields: username
      remoteController: app/admin/handler/auth/admin.go
      remoteModel: app/admin/model/auth/admin.go
`)
	warnings, err := ValidateSpec(path)
	if err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("valid spec produced warnings: %+v", warnings)
	}
}

func TestValidateSpecPrimaryKeyAutoIncrementRequiresUnsigned(t *testing.T) {
	signed := `name: signed_auto_increment
fields:
  - name: id
    type: bigint
    primaryKey: true
    autoIncrement: true
`
	if _, err := ValidateSpec(writeSpecTest(t, signed)); err == nil ||
		!strings.Contains(err.Error(), `field "id"`) ||
		!strings.Contains(err.Error(), "unsigned: true") {
		t.Fatalf("signed auto-increment primary key error = %v", err)
	}

	unsigned := `name: unsigned_auto_increment
fields:
  - name: id
    type: bigint
    primaryKey: true
    autoIncrement: true
    unsigned: true
`
	if _, err := ValidateSpec(writeSpecTest(t, unsigned)); err != nil {
		t.Fatalf("unsigned auto-increment primary key rejected: %v", err)
	}

	nonAutoIncrement := `name: signed_non_auto_increment
fields:
  - name: id
    type: bigint
    primaryKey: true
`
	if _, err := ValidateSpec(writeSpecTest(t, nonAutoIncrement)); err != nil {
		t.Fatalf("non-auto-increment primary key rejected: %v", err)
	}
}

func TestValidateSpecRejectsPrimaryKeyRelationAndRemoteFiles(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "missing primary key",
			yaml: `name: no_primary_key
fields:
  - {name: name, type: varchar}
`,
			want: "exactly one primary key",
		},
		{
			name: "multiple primary keys",
			yaml: `name: multiple_primary_keys
fields:
  - {name: id, type: bigint, primaryKey: true}
  - {name: other_id, type: bigint, primaryKey: true}
`,
			want: "exactly one primary key",
		},
		{
			name: "remote controller",
			yaml: `name: bad_controller
fields:
  - {name: id, type: bigint, primaryKey: true}
  - name: owner_id
    type: bigint
    form:
      remoteController: app/admin/handler/not_found.go
`,
			want: "remoteController",
		},
		{
			name: "remote model",
			yaml: `name: bad_model
fields:
  - {name: id, type: bigint, primaryKey: true}
  - name: owner_id
    type: bigint
    form:
      remoteTable: admin
      remoteModel: app/admin/model/not_found.go
`,
			want: "remoteModel",
		},
		{
			name: "relative path",
			yaml: `name: bad_path
generateRelativePath: ../escape
fields:
  - {name: id, type: bigint, primaryKey: true}
`,
			want: "generateRelativePath",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ValidateSpec(writeSpecTest(t, test.yaml))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestValidateSpecDefaultPairing(t *testing.T) {
	valid := []string{
		`name: inferred_input
fields:
  - {name: id, type: bigint, primaryKey: true}
  - {name: status, type: varchar, default: "enabled"}
`,
		`name: explicit_input
fields:
  - {name: id, type: bigint, primaryKey: true}
  - {name: status, type: varchar, defaultType: input, default: ""}
`,
		`name: explicit_sentinel
fields:
  - {name: id, type: bigint, primaryKey: true}
  - {name: status, type: varchar, defaultType: NONE, default: "none"}
`,
	}
	for index, yaml := range valid {
		t.Run("valid_"+string(rune('a'+index)), func(t *testing.T) {
			if _, err := ValidateSpec(writeSpecTest(t, yaml)); err != nil {
				t.Errorf("valid default pairing rejected: %v", err)
			}
		})
	}

	invalid := []string{
		`name: implicit_blank
fields:
  - {name: id, type: bigint, primaryKey: true}
  - {name: status, type: varchar, default: ""}
`,
		`name: mismatched_none
fields:
  - {name: id, type: bigint, primaryKey: true}
  - {name: status, type: varchar, defaultType: NONE, default: enabled}
`,
		`name: empty_non_input
fields:
  - {name: id, type: bigint, primaryKey: true}
  - {name: status, type: varchar, defaultType: NONE, default: ""}
`,
	}
	for _, yaml := range invalid {
		if _, err := ValidateSpec(writeSpecTest(t, yaml)); err == nil {
			t.Errorf("invalid default pairing accepted: %s", yaml)
		}
	}
}

func TestValidateSpecReportsRouteAndPathWarnings(t *testing.T) {
	routeWarningPath := writeSpecTest(t, `name: route_warning
fields:
  - {name: id, type: bigint, primaryKey: true}
  - name: owner_id
    type: bigint
    form:
      remoteController: app/admin/handler/base.go
`)
	warnings, err := ValidateSpec(routeWarningPath)
	if err != nil {
		t.Fatalf("route warning spec rejected: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "no registered route constant") {
		t.Fatalf("route warning = %+v", warnings)
	}

	pathWarningPath := writeSpecTest(t, `name: seller_money_log
generateRelativePath: seller.moneyLog
fields:
  - {name: id, type: bigint, primaryKey: true}
`)
	warnings, err = ValidateSpec(pathWarningPath)
	if err != nil {
		t.Fatalf("path warning spec rejected: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "uppercase/camel-case") {
		t.Fatalf("path warning = %+v", warnings)
	}

	deepPath := writeSpecTest(t, `name: test_xxx
generateRelativePath: ops/user/test_xxx
fields:
  - {name: id, type: bigint, primaryKey: true}
`)
	warnings, err = ValidateSpec(deepPath)
	if err != nil {
		t.Fatalf("snake-case deep path rejected: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("snake-case deep path warning = %+v", warnings)
	}
}

func TestValidateSpecsAggregatesErrorsAndWarnings(t *testing.T) {
	validPath := writeSpecTest(t, `name: aggregate_warning
fields:
  - {name: id, type: bigint, primaryKey: true}
  - name: owner_id
    type: bigint
    form:
      remoteController: app/admin/handler/base.go
`)
	invalidPath := writeSpecTest(t, `name: aggregate_error
fields:
  - {name: name, type: varchar}
`)
	warnings, err := ValidateSpecs([]string{validPath, invalidPath})
	if err == nil || !strings.Contains(err.Error(), invalidPath) {
		t.Fatalf("aggregate error = %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "no registered route constant") {
		t.Fatalf("aggregate warnings = %+v", warnings)
	}
}
