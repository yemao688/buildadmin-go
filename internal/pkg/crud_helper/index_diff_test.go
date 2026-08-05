package crud_helper

import (
	"path/filepath"
	"strings"
	"testing"

	crudmodel "buildadmin-go/internal/model"
)

func TestParseSpecIndexesValid(t *testing.T) {
	fields := []crudmodel.Field{{Name: "id", PrimaryKey: true}, {Name: "order_no"}, {Name: "seller_id"}, {Name: "hotel_id"}}
	raw := []specIndex{
		{Name: "uk_order_no", Unique: true, Columns: []string{"order_no"}},
		{Name: "idx_seller_hotel", Columns: []string{"seller_id", "hotel_id"}},
	}
	indexes, err := parseSpecIndexes(raw, fields)
	if err != nil {
		t.Fatal(err)
	}
	if len(indexes) != 2 {
		t.Fatalf("indexes = %d, want 2", len(indexes))
	}
	if !indexes[0].Unique || indexes[0].Name != "uk_order_no" || indexes[0].Columns[0] != "order_no" {
		t.Fatalf("index 0 = %+v", indexes[0])
	}
	if indexes[1].Unique {
		t.Fatalf("index 1 should default to non-unique: %+v", indexes[1])
	}
}

func TestParseSpecIndexesRejects(t *testing.T) {
	fields := []crudmodel.Field{{Name: "id", PrimaryKey: true}, {Name: "order_no"}}
	cases := []struct {
		name  string
		index specIndex
	}{
		{"empty name", specIndex{Columns: []string{"order_no"}}},
		{"unknown column", specIndex{Name: "uk_x", Columns: []string{"nope"}}},
		{"duplicate column", specIndex{Name: "uk_x", Columns: []string{"order_no", "order_no"}}},
		{"bad identifier", specIndex{Name: "uk bad", Columns: []string{"order_no"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseSpecIndexes([]specIndex{tc.index}, fields); err == nil {
				t.Fatalf("parseSpecIndexes(%+v) = nil error, want error", tc.index)
			}
		})
	}
	if _, err := parseSpecIndexes([]specIndex{
		{Name: "uk_x", Columns: []string{"order_no"}},
		{Name: "uk_x", Columns: []string{"order_no"}},
	}, fields); err == nil {
		t.Fatal("duplicate index names accepted")
	}
}

func TestCreateTableDDLIncludesIndexes(t *testing.T) {
	table := crudmodel.Table{
		Name: "orders",
		Indexes: []crudmodel.IndexSpec{
			{Name: "uk_order_no", Unique: true, Columns: []string{"order_no"}},
			{Name: "idx_seller_hotel", Columns: []string{"seller_id", "hotel_id"}},
		},
	}
	fields := []crudmodel.Field{{Name: "id", Type: "bigint", PrimaryKey: true}, {Name: "order_no", Type: "varchar"}, {Name: "seller_id", Type: "bigint"}, {Name: "hotel_id", Type: "bigint"}}
	ddl, err := createTableDDL("ba_orders", table, fields)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, "UNIQUE KEY `uk_order_no` (`order_no`)") {
		t.Fatalf("unique key missing: %s", ddl)
	}
	if !strings.Contains(ddl, "KEY `idx_seller_hotel` (`seller_id`, `hotel_id`)") {
		t.Fatalf("plain key missing: %s", ddl)
	}
	if !strings.Contains(ddl, "PRIMARY KEY (`id`)") {
		t.Fatalf("primary key missing: %s", ddl)
	}
}

func TestDeriveIndexDiffs(t *testing.T) {
	wanted := []crudmodel.IndexSpec{
		{Name: "uk_order_no", Unique: true, Columns: []string{"order_no"}},
		{Name: "uk_seller_hotel", Unique: true, Columns: []string{"seller_id", "hotel_id"}},
	}
	actual := []actualIndex{
		{Name: "PRIMARY", Unique: true, Columns: []string{"id"}},
		{Name: "idx_seller_id", Columns: []string{"seller_id"}}, // data_scope 机制索引，豁免
		{Name: "uk_order_no", Unique: true, Columns: []string{"order_no"}},
		{Name: "legacy_idx", Columns: []string{"create_time"}}, // 线外索引，unmanaged
	}
	plan := deriveIndexDiffs(actual, wanted, "ba_orders")
	if len(plan.Added) != 1 || plan.Added[0].Field != "uk_seller_hotel" {
		t.Fatalf("added = %+v, want uk_seller_hotel", plan.Added)
	}
	if plan.Added[0].Class != DiffSafeAuto {
		t.Fatalf("added class = %s, want safe-auto", plan.Added[0].Class)
	}
	wantDDL := "ALTER TABLE `ba_orders` ADD UNIQUE INDEX `uk_seller_hotel` (`seller_id`, `hotel_id`)"
	if plan.Added[0].DDL != wantDDL {
		t.Fatalf("DDL = %q, want %q", plan.Added[0].DDL, wantDDL)
	}
	if len(plan.Unmanaged) != 1 || plan.Unmanaged[0].Field != "legacy_idx" {
		t.Fatalf("unmanaged = %+v, want legacy_idx only", plan.Unmanaged)
	}
}

func TestDeriveIndexDiffsDefinitionDrift(t *testing.T) {
	wanted := []crudmodel.IndexSpec{{Name: "uk_order_no", Unique: true, Columns: []string{"order_no"}}}
	actual := []actualIndex{{Name: "uk_order_no", Unique: false, Columns: []string{"order_no"}}}
	plan := deriveIndexDiffs(actual, wanted, "`t`")
	if len(plan.Added) != 0 {
		t.Fatalf("added = %+v, want none", plan.Added)
	}
	if len(plan.Unmanaged) != 1 || !strings.Contains(plan.Unmanaged[0].Reason, "definition differs") {
		t.Fatalf("unmanaged = %+v, want definition-drift warning", plan.Unmanaged)
	}
}

func TestSpecIndexesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "orders.yaml")
	content := `name: orders
comment: 订单
fields:
  - {name: id, type: bigint, primaryKey: true}
  - name: order_no
    type: varchar
    length: 64
  - name: seller_id
    type: bigint
  - name: hotel_id
    type: bigint
indexes:
  - name: uk_order_no
    unique: true
    columns: [order_no]
  - name: uk_seller_hotel
    unique: true
    columns: [seller_id, hotel_id]
`
	if err := writeFile(specPath, content); err != nil {
		t.Fatal(err)
	}
	opts, err := LoadSpec(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.Table.Indexes) != 2 {
		t.Fatalf("indexes = %d, want 2", len(opts.Table.Indexes))
	}
	if opts.Table.Indexes[0].Name != "uk_order_no" || !opts.Table.Indexes[0].Unique {
		t.Fatalf("index 0 = %+v", opts.Table.Indexes[0])
	}
}
