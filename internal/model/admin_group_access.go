package model

// AdminGroupAccess 管理员分组关联表
type AdminGroupAccess struct {
	UID     int32 `gorm:"column:uid;type:int(11) unsigned;not null;index:uid;comment:管理员ID" json:"uid"`               // 管理员ID
	GroupID int32 `gorm:"column:group_id;type:int(11) unsigned;not null;index:group_id;comment:分组ID" json:"group_id"` // 分组ID
}
