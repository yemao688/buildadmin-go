package model

const TableNameAdminGroup = "ba_admin_group"

type AdminGroup struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // ID
	Pid        int32  `gorm:"column:pid;not null;comment:上级分组" json:"pid"`                         // 上级分组
	Name       string `gorm:"column:name;not null;comment:组名" json:"name"`                         // 组名
	Rules      string `gorm:"column:rules;comment:权限规则ID" json:"rules"`                            // 权限规则ID
	Status     string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	UpdateTime int64  `gorm:"column:update_time;comment:更新时间" json:"update_time"`                  // 更新时间
	CreateTime int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`                  // 创建时间
}

func (*AdminGroup) TableName() string {
	return TableNameAdminGroup
}
