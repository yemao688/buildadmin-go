package model

const TableNameAdminGroupAccess = "ba_admin_group_access"

type AdminGroupAccess struct {
	UID     int32 `gorm:"column:uid;not null;comment:管理员ID" json:"uid"`          // 管理员ID
	GroupID int32 `gorm:"column:group_id;not null;comment:分组ID" json:"group_id"` // 分组ID
}

func (*AdminGroupAccess) TableName() string {
	return TableNameAdminGroupAccess
}
