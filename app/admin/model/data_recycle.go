package model

const TableNameSecurityDataRecycle = "ba_security_data_recycle"

// SecurityDataRecycle 回收规则表
type SecurityDataRecycle struct {
	ID           int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // ID
	Name         string `gorm:"column:name;not null;comment:规则名称" json:"name"`                       // 规则名称
	Controller   string `gorm:"column:controller;not null;comment:控制器" json:"controller"`            // 控制器
	ControllerAs string `gorm:"column:controller_as;not null;comment:控制器别名" json:"controller_as"`    // 控制器别名
	DataTable    string `gorm:"column:data_table;not null;comment:对应数据表" json:"data_table"`          // 对应数据表
	PrimaryKey   string `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`        // 数据表主键
	Status       string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	UpdateTime   int64  `gorm:"column:update_time;comment:更新时间" json:"update_time"`                  // 更新时间
	CreateTime   int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`                  // 创建时间
}
