package model

import (
	"go-build-admin/utils"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameUserScoreLog = "ba_user_score_log"

// UserScoreLog 会员积分变动表
type UserScoreLog struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`      // ID
	UserID     int32  `gorm:"column:user_id;not null;comment:会员ID" json:"user_id"`               // 会员ID
	Score      int32  `gorm:"column:score;not null;comment:变更积分" json:"score"`                   // 变更积分
	Before     int32  `gorm:"column:before;not null;comment:变更前积分" json:"before"`                // 变更前积分
	After      int32  `gorm:"column:after;not null;comment:变更后积分" json:"after"`                  // 变更后积分
	Memo       string `gorm:"column:memo;not null;comment:备注" json:"memo"`                       // 备注
	CreateTime int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
}

func (*UserScoreLog) TableName() string {
	return TableNameUserScoreLog
}

type UserScoreLogModel struct {
	sqlDB *gorm.DB
}

func NewUserScoreLogModel(sqlDB *gorm.DB) *UserScoreLogModel {
	return &UserScoreLogModel{
		sqlDB: sqlDB,
	}
}

func (s *UserScoreLogModel) GetDayScore(ctx *gin.Context, t time.Time, userId int) (int, error) {
	type Result struct {
		Total int
	}
	startUnix := utils.DayStartUnix(t)
	endUnix := utils.DayEndUnix(t)
	var result = Result{}
	err := s.sqlDB.Model(&UserScoreLog{}).
		Select("sum(score) as total").
		Where("user_id=?", userId).
		Where("created_at BETWEEN ? AND ?", startUnix, endUnix).
		First(&result).Error

	return result.Total, err
}

func (s *UserScoreLogModel) List(ctx *gin.Context, userId int) (list []UserScoreLog, total int64, err error) {
	limit, offset := LimitAddOffset(ctx)
	db := s.sqlDB.Model(&UserScoreLog{}).Where("user_id=?", userId)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Limit(limit).Offset(offset).Find(&list).Error
	return
}
