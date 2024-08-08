package crud_helper

import "go-build-admin/app/admin/model"

// 创建表或更新表
func HandleTableDesign(table model.JSON_TABLE, fields []model.Field) string {
	// tableName := h.tableM.Name(table.Name, true)
	// comment := table.Comment
	// designChange := table.DesignChange

	// pk := "id"
	// for _, v := range fields {
	// 	if v.PrimaryKey == "1" {
	// 		pk = v.Name
	// 		break
	// 	}
	// }

	// h.db.Migrator().HasTable()
	// if h.tableM.IsExist(tableName) {

	// } else {
	// 	//创建表

	// }
	return ""
}

func getPhinxFieldType() {

}

func searchArray() {

}

func getPhinxFieldData() {

}

func updateFieldOrder() {

}

// 根据数据表解析字段数据
func parseTableColumns(tableModel *model.TableModel, tableName string, analyseField bool) {
	//从数据库中获取表字段信息
	// sql := "SELECT * FROM `information_schema`.`columns`  WHERE TABLE_SCHEMA = ? AND table_name = ? ORDER BY ORDINAL_POSITION"

}

// 解析到的表字段的额外处理
func handleTableColumn() {
	// 预留
}

func getTableColumnsDataType(field model.Field) {

}

func isMatchSuffix() {

}

func AnalyseFieldDefault(field model.Field) {

}
