package model

import (
	"fmt"
	"go-build-admin/utils"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UserMoneyLog 余额变动表
type UserMoneyLog struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`      // ID
	UserID     int32  `gorm:"column:user_id;not null;comment:会员ID" json:"user_id"`               // 会员ID
	Money      int32  `gorm:"column:money;not null;comment:变更余额" json:"money"`                   // 变更余额
	Before     int32  `gorm:"column:before;not null;comment:变更前余额" json:"before"`                // 变更前余额
	After      int32  `gorm:"column:after;not null;comment:变更后余额" json:"after"`                  // 变更后余额
	Memo       string `gorm:"column:memo;not null;comment:备注" json:"memo"`                       // 备注
	CreateTime int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
}

type UserMoneyLogModel struct {
	sqlDB *gorm.DB
}

func NewUserMoneyLogModel(sqlDB *gorm.DB) *UserMoneyLogModel {
	return &UserMoneyLogModel{
		sqlDB: sqlDB,
	}
}

func (s *UserMoneyLogModel) GetDayMoney(ctx *gin.Context, t time.Time, userId int32) (int, error) {
	type Result struct {
		Total int
	}
	startUnix := utils.DayStartUnix(t)
	endUnix := utils.DayEndUnix(t)
	var result = Result{}
	err := s.sqlDB.Model(&UserMoneyLog{}).
		Select("sum(money) as total").
		Where("user_id=?", userId).
		Where("create_time BETWEEN ? AND ?", startUnix, endUnix).
		First(&result).Error

	return result.Total, err
}

func (s *UserMoneyLogModel) List(ctx *gin.Context, userId int32) (result []map[string]any, total int64, err error) {
	limit, offset := LimitAddOffset(ctx)
	db := s.sqlDB.Model(&UserMoneyLog{}).Where("user_id=?", userId)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	list := []*UserMoneyLog{}
	err = db.Limit(limit).Offset(offset).Find(&list).Error

	for _, v := range list {
		result = append(result, map[string]any{
			"user_id":     v.UserID,
			"money":       fmt.Sprintf("%.2f", float64(v.Money/100)),
			"before":      fmt.Sprintf("%.2f", float64(v.Before/100)),
			"after":       fmt.Sprintf("%.2f", float64(v.After/100)),
			"memo":        v.Memo,
			"create_time": v.CreateTime,
		})
	}

	return
}
