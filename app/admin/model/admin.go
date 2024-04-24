package model

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameAdmin = "ba_admin"

type Admin struct {
	ID            int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`      // ID
	Username      string `gorm:"column:username;not null;comment:用户名" json:"username"`              // 用户名
	Nickname      string `gorm:"column:nickname;not null;comment:昵称" json:"nickname"`               // 昵称
	Avatar        string `gorm:"column:avatar;not null;comment:头像" json:"avatar"`                   // 头像
	Email         string `gorm:"column:email;not null;comment:邮箱" json:"email"`                     // 邮箱
	Mobile        string `gorm:"column:mobile;not null;comment:手机" json:"mobile"`                   // 手机
	LoginFailure  int32  `gorm:"column:login_failure;not null;comment:登录失败次数" json:"login_failure"` // 登录失败次数
	LastLoginTime int64  `gorm:"column:last_login_time;comment:上次登录时间" json:"last_login_time"`      // 上次登录时间
	LastLoginIP   string `gorm:"column:last_login_ip;not null;comment:上次登录IP" json:"last_login_ip"` // 上次登录IP
	Password      string `gorm:"column:password;not null;comment:密码" json:"password"`               // 密码
	Salt          string `gorm:"column:salt;not null;comment:密码盐" json:"salt"`                      // 密码盐
	Motto         string `gorm:"column:motto;not null;comment:签名" json:"motto"`                     // 签名
	RongToken     string `gorm:"column:rong_token" json:"rong_token"`
	TeamID        int32  `gorm:"column:team_id;comment:团队id" json:"team_id"`                          // 团队id
	Online        int32  `gorm:"column:online;comment:0:下线,1:在线,2:离线" json:"online"`                  // 0:下线,1:在线,2:离线
	Status        string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	UpdateTime    int64  `gorm:"column:update_time;comment:更新时间" json:"update_time"`                  // 更新时间
	CreateTime    int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`                  // 创建时间
}

func (Admin) TableName() string {
	return TableNameAdmin
}

func (Admin) Key() string {
	return "id"
}

func (Admin) QuickSearchField() string {
	return "id"
}

type AdminModel struct {
	sqlDB *gorm.DB
}

func NewAdminModel(sqlDB *gorm.DB) *AdminModel {
	return &AdminModel{sqlDB: sqlDB}
}

func (s *AdminModel) List(ctx *gin.Context, id int32) (list []Admin, err error) {
	var admin Admin
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, admin, nil)
	if err != nil {
		return nil, err
	}
	err = s.sqlDB.Table(TableNameAdmin).Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}
