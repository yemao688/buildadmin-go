package simple

type Admin struct {
	ID       int32  `gorm:"column:id;type:int(11) unsigned;primaryKey;autoIncrement:true;comment:ID" json:"id"` // ID
	Username string `gorm:"column:username;not null;comment:用户名" json:"username"`
	Nickname string `gorm:"column:nickname;not null;comment:昵称" json:"nickname"`
}
