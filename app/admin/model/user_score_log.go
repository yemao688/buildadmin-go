package model

import (
	"go-build-admin/app/admin/model/simple"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UserScoreLog 会员积分变动表
type UserScoreLog struct {
	ID         int32      `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`      // ID
	UserID     int32      `gorm:"column:user_id;not null;comment:会员ID" json:"user_id"`               // 会员ID
	Score      int32      `gorm:"column:score;not null;comment:变更积分" json:"score"`                   // 变更积分
	Before     int32      `gorm:"column:before;not null;comment:变更前积分" json:"before"`                // 变更前积分
	After      int32      `gorm:"column:after;not null;comment:变更后积分" json:"after"`                  // 变更后积分
	Memo       string     `gorm:"column:memo;not null;comment:备注" json:"memo"`                       // 备注
	CreateTime int64      `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
	User       simple.User `json:"user"`
}

type UserScoreLogModel struct {
	BaseModel
}

func NewUserScoreLogModel(sqlDB *gorm.DB, config *conf.Configuration) *UserScoreLogModel {
	return &UserScoreLogModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "user_score_log",
			Key:              "id",
			QuickSearchField: "user.username,user.nickname",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *UserScoreLogModel) List(ctx *gin.Context) (list []UserScoreLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.sqlDB.Model(&UserScoreLog{}).Preload("User").Joins("User").Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *UserScoreLogModel) Add(ctx *gin.Context, userScoreLog UserScoreLog) error {
	user := User{}
	if err := s.sqlDB.Where("id=?", userScoreLog.UserID).Take(&user).Error; err != nil {
		return err
	}

	userScoreLog.Before = user.Score
	userScoreLog.After = user.Score + userScoreLog.Score

	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&User{}).Where("id=?", userScoreLog.UserID).UpdateColumn("score", gorm.Expr("score + ?", userScoreLog.Score)).Error; err != nil {
		tx.Rollback()
		return err

	}

	if err := tx.Create(&userScoreLog).Error; err != nil {
		tx.Rollback()
		return err

	}
	return tx.Commit().Error
}
