package simple

type User struct {
	ID       int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`
	Username string `gorm:"column:username;not null;comment:用户名" json:"username"`
	Nickname string `gorm:"column:nickname;not null;comment:昵称" json:"nickname"`
}
