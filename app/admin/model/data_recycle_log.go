package model

const TableNameSecurityDataRecycleLog = "ba_security_data_recycle_log"

// SecurityDataRecycleLog 数据回收记录表
type SecurityDataRecycleLog struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`       // ID
	AdminID    int32  `gorm:"column:admin_id;not null;comment:操作管理员" json:"admin_id"`             // 操作管理员
	RecycleID  int32  `gorm:"column:recycle_id;not null;comment:回收规则ID" json:"recycle_id"`        // 回收规则ID
	Data       string `gorm:"column:data;comment:回收的数据" json:"data"`                              // 回收的数据
	DataTable  string `gorm:"column:data_table;not null;comment:数据表" json:"data_table"`           // 数据表
	PrimaryKey string `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`       // 数据表主键
	IsRestore  int32  `gorm:"column:is_restore;not null;comment:是否已还原:0=否,1=是" json:"is_restore"` // 是否已还原:0=否,1=是
	IP         string `gorm:"column:ip;not null;comment:操作者IP" json:"ip"`                         // 操作者IP
	Useragent  string `gorm:"column:useragent;not null;comment:User-Agent" json:"useragent"`      // User-Agent
	CreateTime int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`                 // 创建时间
}

// TableName SecurityDataRecycleLog's table name
func (*SecurityDataRecycleLog) TableName() string {
	return TableNameSecurityDataRecycleLog
}
