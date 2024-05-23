package model

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameUserMoneyLog = "ba_user_money_log"

// UserMoneyLog 会员余额变动表
type UserMoneyLog struct {
	ID         int32      `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`      // ID
	UserID     int32      `gorm:"column:user_id;not null;comment:会员ID" json:"user_id"`               // 会员ID
	Money      int32      `gorm:"column:money;not null;comment:变更余额" json:"money"`                   // 变更余额
	Before     int32      `gorm:"column:before;not null;comment:变更前余额" json:"before"`                // 变更前余额
	After      int32      `gorm:"column:after;not null;comment:变更后余额" json:"after"`                  // 变更后余额
	Memo       string     `gorm:"column:memo;not null;comment:备注" json:"memo"`                       // 备注
	CreateTime int64      `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
	User       SimpleUser `json:"user"`
}

func (*UserMoneyLog) TableName() string {
	return TableNameUserMoneyLog
}

type UserMoneyLogModel struct {
	BaseModel
}

func NewUserMoneyLogModel(sqlDB *gorm.DB) *UserMoneyLogModel {
	return &UserMoneyLogModel{
		BaseModel: BaseModel{
			TableName:        TableNameUserMoneyLog,
			Key:              "id",
			QuickSearchField: "id",
			DataLimit:        TableNameUser + ".username," + TableNameUser + ".nickname",
			sqlDB:            sqlDB,
		},
	}
}

func (s *UserMoneyLogModel) List(ctx *gin.Context) (list []UserMoneyLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	//预加载需要使用Model
	db := s.sqlDB.Model(&UserMoneyLog{}).Preload("User").Scopes(IsSuperAdmin(ctx)).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *UserMoneyLogModel) Add(ctx *gin.Context, userMoneyLog UserMoneyLog) error {
	user := User{}
	if err := s.sqlDB.Table(TableNameUser).Where("id=?", userMoneyLog.UserID).Take(&user).Error; err != nil {
		return err
	}

	userMoneyLog.Before = user.Money
	userMoneyLog.After = user.Money + userMoneyLog.Money*100

	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Table(TableNameUser).Where("id=?", userMoneyLog.UserID).UpdateColumn("money", gorm.Expr("money + ?", userMoneyLog.Money*100)).Error; err != nil {
		tx.Rollback()
		return err

	}

	if err := tx.Table(s.TableName).Create(&userMoneyLog).Error; err != nil {
		tx.Rollback()
		return err

	}
	return tx.Commit().Error
}
