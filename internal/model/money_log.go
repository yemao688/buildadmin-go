package model

import (
	"go-build-admin/internal/admin/repository/simple"

	"gorm.io/gorm/schema"
)

// TableName 经全局命名策略解析（前缀安全），对齐真实表 user_money_log。
func (MoneyLog) TableName(namer schema.Namer) string {
	return namer.TableName("user_money_log")
}

// MoneyLog 会员余额变动表
type MoneyLog struct {
	ID         int32        `gorm:"column:id;type:int(11) unsigned;not null;primaryKey;autoIncrement:true;comment:ID" json:"id"`               // ID
	AdminID    int32        `gorm:"column:admin_id;type:int(11) unsigned;not null;default:0;index:idx_admin_id;comment:管理员ID" json:"admin_id"` // 管理员ID
	UserID     int32        `gorm:"column:user_id;type:int(11) unsigned;not null;default:0;index:idx_user_id;comment:会员ID" json:"user_id"`     // 会员ID
	Money      float64      `gorm:"column:money;type:decimal(12,2);not null;default:0.00;comment:变更余额" json:"money"`                           // 变更余额
	Before     float64      `gorm:"column:before;type:decimal(12,2);not null;default:0.00;comment:变更前余额" json:"before"`                        // 变更前余额
	After      float64      `gorm:"column:after;type:decimal(12,2);not null;default:0.00;comment:变更后余额" json:"after"`                          // 变更后余额
	Memo       string       `gorm:"column:memo;type:varchar(255) default '';not null;comment:备注" json:"memo"`                                  // 备注
	CreateTime int64        `gorm:"autoCreateTime;column:create_time;type:bigint(16) unsigned default null;comment:创建时间" json:"create_time"`   // 创建时间
	Admin      simple.Admin `gorm:"foreignKey:AdminID" json:"admin"`
	User       simple.User  `json:"user"`
}
