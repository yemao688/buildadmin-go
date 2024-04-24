package model

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-build-admin/app/pkg/token"
)

type AuthInfo struct {
	ID            int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"` // ID
	Username      string `gorm:"column:username;not null;comment:用户名" json:"username"`         // 用户名
	Nickname      string `gorm:"column:nickname;not null;comment:昵称" json:"nickname"`          // 昵称
	Avatar        string `gorm:"column:avatar;not null;comment:头像" json:"avatar"`              // 头像
	LastLoginTime int64  `gorm:"column:last_login_time;comment:上次登录时间" json:"last_login_time"` // 上次登录时间
}

type AuthModel struct {
	sqlDB       *gorm.DB
	tokenHelper *token.TokenHelper
}

func NewAuthModel(sqlDB *gorm.DB, tokenHelper *token.TokenHelper) *AuthModel {
	return &AuthModel{sqlDB: sqlDB, tokenHelper: tokenHelper}
}

func (s *AuthModel) GetInfo(ctx *gin.Context, id int32) (list []Admin, err error) {

	return
}

func (s *AuthModel) IsSuperAdmin(ctx *gin.Context, id int32) (list []Admin, err error) {

	return
}

func (s *AuthModel) GetMenus(ctx *gin.Context, id int32) (list []AdminRule, err error) {

	return
}
