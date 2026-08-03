package model

// AdminGroup 管理员分组表
type AdminGroup struct {
	ID         int32  `gorm:"column:id;type:int(11) unsigned;not null;primaryKey;autoIncrement:true;comment:ID" json:"id"`             // ID
	Pid        int32  `gorm:"column:pid;type:int(11) unsigned;not null;default:0;comment:上级分组" json:"pid"`                             // 上级分组
	Name       string `gorm:"column:name;type:varchar(100) default '';not null;comment:组名" json:"name"`                                // 组名
	Rules      string `gorm:"column:rules;type:text;comment:权限规则ID" json:"rules"`                                                      // 权限规则ID
	Status     string `gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"`                  // 状态:0=禁用,1=启用
	UpdateTime int64  `gorm:"autoUpdateTime;column:update_time;type:bigint(16) unsigned default null;comment:更新时间" json:"update_time"` // 更新时间
	CreateTime int64  `gorm:"autoCreateTime;column:create_time;type:bigint(16) unsigned default null;comment:创建时间" json:"create_time"` // 创建时间
}
