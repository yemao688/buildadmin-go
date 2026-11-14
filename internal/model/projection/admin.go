// Package projection 存放按渠道裁剪的实体投影（如 Admin/User 视图），避免共享实体直接泄露渠道字段。
package projection

type Admin struct {
	ID       int32  `gorm:"column:id;type:int(11) unsigned;primaryKey;autoIncrement:true;comment:ID" json:"id"` // ID
	Username string `gorm:"column:username;not null;comment:用户名" json:"username"`
	Nickname string `gorm:"column:nickname;not null;comment:昵称" json:"nickname"`
}
