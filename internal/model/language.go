package model

// Language 全局语言
type Language struct {
	ID     int64  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // 主键
	Lan    string `gorm:"column:lan;not null;comment:语言代码" json:"lan"`                         // 语言代码
	Name   string `gorm:"column:name;not null;comment:语言名称" json:"name"`                       // 语言名称
	Remark string `gorm:"column:remark;not null;comment:备注" json:"remark"`                     // 备注
	Status int32  `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	Weigh  int32  `gorm:"column:weigh;not null;comment:权重" json:"weigh"`                       // 权重
}
