package crud_helper

// 表级索引的 apply 差量。spec 通过 `indexes:` 声明唯一/普通索引，
// apply 建表时内联物化（createTableDDL），已有表时按此 diff 同步：
// spec 有而实际无 → safe-auto 新增；实际有而 spec 无 → unmanaged 保留并告警；
// 同名但列定义不同 → unmanaged 保留并告警（不自动重建，防数据风险）。

import (
	"fmt"
	"sort"
	"strings"

	crudmodel "buildadmin-go/internal/model"

	"gorm.io/gorm"
)

// actualIndex 是 information_schema.statistics 按索引名分组后的实际形态。
type actualIndex struct {
	Name    string
	Unique  bool
	Columns []string
}

// readActualIndexes 读取表的全部索引（含主键与框架机制索引，如 data_scope 的 idx_*）。
func readActualIndexes(db *gorm.DB, fullTableName string) ([]actualIndex, error) {
	type statRow struct {
		IndexName  string
		NonUnique  int64
		ColumnName string
		SeqInIndex int64
	}
	var rows []statRow
	if err := db.Raw(`SELECT INDEX_NAME AS index_name, NON_UNIQUE AS non_unique, COLUMN_NAME AS column_name, SEQ_IN_INDEX AS seq_in_index
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY index_name, seq_in_index`, fullTableName).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read indexes of %s: %w", fullTableName, err)
	}
	byName := make(map[string]*actualIndex)
	var order []string
	for _, row := range rows {
		idx, ok := byName[row.IndexName]
		if !ok {
			idx = &actualIndex{Name: row.IndexName, Unique: row.NonUnique == 0}
			byName[row.IndexName] = idx
			order = append(order, row.IndexName)
		}
		idx.Columns = append(idx.Columns, row.ColumnName)
	}
	sort.Strings(order)
	indexes := make([]actualIndex, 0, len(order))
	for _, name := range order {
		indexes = append(indexes, *byName[name])
	}
	return indexes, nil
}

type indexDiffPlan struct {
	Added     []ApplyChange // SafeAuto，需要执行 ADD
	Unmanaged []ApplyChange // 保留 + 告警
}

// deriveIndexDiffs 对比实际索引与 spec 声明（纯函数，便于测试）。
// dataScopeIndexName 是框架 data_scope 机制索引的精确名（"idx_"+ownerColumn，
// 无 owner 时为空串），只豁免该名；其它 idx_* 前缀索引一律视为线外索引告警。
func deriveIndexDiffs(actual []actualIndex, dataScopeIndexName string, wanted []crudmodel.IndexSpec, fullTableName string) indexDiffPlan {
	var plan indexDiffPlan
	actualByName := make(map[string]actualIndex, len(actual))
	wantedByName := make(map[string]crudmodel.IndexSpec, len(wanted))
	for _, idx := range actual {
		actualByName[idx.Name] = idx
	}
	for _, idx := range wanted {
		wantedByName[idx.Name] = idx
	}
	// spec 声明 → 实际缺失则新增；同名但定义不同则保留并告警。
	for _, idx := range wanted {
		// 与 data_scope 机制索引同名：由 EnsureDataScopeIndex 负责，apply 不重复建。
		if idx.Name == dataScopeIndexName && dataScopeIndexName != "" {
			continue
		}
		actualIdx, ok := actualByName[idx.Name]
		if !ok {
			plan.Added = append(plan.Added, ApplyChange{
				Field: idx.Name, Type: "add-index", Class: DiffSafeAuto,
				DDL:    indexAddDDL(fullTableName, idx),
				Reason: "index declared in spec but missing in database",
			})
			continue
		}
		if !sameIndexDefinition(actualIdx, idx) {
			plan.Unmanaged = append(plan.Unmanaged, ApplyChange{
				Field: idx.Name, Type: "index-drift", Class: DiffUnmanaged,
				Reason: fmt.Sprintf("index %q definition differs (spec=%v unique=%t; database=%v unique=%t) and is left untouched", idx.Name, idx.Columns, idx.Unique, actualIdx.Columns, actualIdx.Unique),
			})
		}
	}
	// 实际存在但 spec 未声明：主键与 data_scope 机制索引不算漂移，其余保留并告警。
	for _, idx := range actual {
		if _, ok := wantedByName[idx.Name]; ok {
			continue
		}
		if idx.Name == "PRIMARY" || (dataScopeIndexName != "" && idx.Name == dataScopeIndexName) {
			continue
		}
		plan.Unmanaged = append(plan.Unmanaged, ApplyChange{
			Field: idx.Name, Type: "unmanaged-index", Class: DiffUnmanaged,
			Reason: fmt.Sprintf("index %q exists in database but is not declared in spec (kept)", idx.Name),
		})
	}
	return plan
}

func sameIndexDefinition(actual actualIndex, spec crudmodel.IndexSpec) bool {
	if actual.Unique != spec.Unique || len(actual.Columns) != len(spec.Columns) {
		return false
	}
	for i, col := range spec.Columns {
		if actual.Columns[i] != col {
			return false
		}
	}
	return true
}

func indexAddDDL(fullTableName string, idx crudmodel.IndexSpec) string {
	columns := make([]string, 0, len(idx.Columns))
	for _, col := range idx.Columns {
		columns = append(columns, "`"+col+"`")
	}
	keyWord := "INDEX"
	if idx.Unique {
		keyWord = "UNIQUE INDEX"
	}
	return fmt.Sprintf("ALTER TABLE `%s` ADD %s `%s` (%s)", fullTableName, keyWord, idx.Name, strings.Join(columns, ", "))
}
