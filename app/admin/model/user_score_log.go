package model

const TableNameUserScoreLog = "ba_user_score_log"

// UserScoreLog 会员积分变动表
type UserScoreLog struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"` // ID
	UserID     int32  `gorm:"column:user_id;not null;comment:会员ID" json:"user_id"`          // 会员ID
	Score      int32  `gorm:"column:score;not null;comment:变更积分" json:"score"`              // 变更积分
	Before     int32  `gorm:"column:before;not null;comment:变更前积分" json:"before"`           // 变更前积分
	After      int32  `gorm:"column:after;not null;comment:变更后积分" json:"after"`             // 变更后积分
	Memo       string `gorm:"column:memo;not null;comment:备注" json:"memo"`                  // 备注
	CreateTime int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`           // 创建时间
}
