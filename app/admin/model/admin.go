package model

import (
	"go-build-admin/app/pkg/random"
	"go-build-admin/utils"

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

type AdminExpand struct {
	Admin
	GroupArr     []int32  `json:"group_arr"`
	GroupNameArr []string `json:"group_name_arr"`
}

type AdminModel struct {
	BaseModel
	sqlDB *gorm.DB
}

func NewAdminModel(sqlDB *gorm.DB) *AdminModel {
	return &AdminModel{
		BaseModel: BaseModel{
			TableName:        TableNameAdmin,
			Key:              "id",
			QuickSearchField: "id",
			DataLimit:        "",
		},
		sqlDB: sqlDB,
	}
}

func (s *AdminModel) GetOne(ctx *gin.Context, id int32) (admin Admin, err error) {
	err = s.sqlDB.Table(s.TableName).Omit("password,salt,login_failure").Where("id=?", id).Limit(1).First(&admin).Error
	return
}

func (s *AdminModel) GetGroupArr(ctx *gin.Context, id int32) (groupIds []int32, err error) {
	err = s.sqlDB.Table(TableNameAdminGroupAccess).Where("uid=?", id).Pluck("group_id", &groupIds).Error
	return
}

func (s *AdminModel) GetGroupNameArr(ctx *gin.Context, id int32) (groupNames []string, err error) {
	err = s.sqlDB.Table(TableNameAdminGroupAccess).
		Joins("left join ba_admin_group on ba_admin_group_access.group_id = ba_admin_group.id").Where("uid=?", id).Pluck("name", &groupNames).Error
	return
}

func (s *AdminModel) List(ctx *gin.Context) (list []Admin, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	err = s.sqlDB.Table(s.TableName).Scopes(Total(whereS, whereP, &total)).Omit("password,salt,login_failure").Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *AdminModel) Add(ctx *gin.Context, admin Admin, groups []string) error {
	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Table(s.TableName).Omit("login_failure, last_login_time, last_login_ip").Create(&admin).Error; err != nil {
		tx.Rollback()
		return err

	}

	access := []map[string]interface{}{}
	for _, v := range groups {
		access = append(access, map[string]interface{}{
			"uid": admin.ID, "group_id": v,
		})
	}

	if err := tx.Table(TableNameAdminGroupAccess).Create(access).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *AdminModel) Edit(ctx *gin.Context, admin Admin, omit string, groups []string) error {
	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Table(s.TableName).Omit(omit).Save(&admin).Error; err != nil {
		tx.Rollback()
		return err

	}

	if len(groups) > 0 {
		if err := tx.Table(TableNameAdminGroupAccess).Where("uid=?", admin.ID).Delete(nil).Error; err != nil {
			tx.Rollback()
			return err
		}
		access := []map[string]interface{}{}
		for _, v := range groups {
			access = append(access, map[string]interface{}{
				"uid": admin.ID, "group_id": v,
			})
		}

		if err := tx.Table(TableNameAdminGroupAccess).Create(access).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

func (s *AdminModel) ResetPassword(ctx *gin.Context, id int32, password string) error {
	salt := random.Build("alnum", 16)
	password = utils.EncryptPassword(password, salt)
	err := s.sqlDB.Table(s.TableName).Where("id=?", id).Updates(map[string]interface{}{
		"salt":     salt,
		"password": password,
	}).Error
	return err
}

func (s *AdminModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}

func (s *AdminModel) Sortable(ctx *gin.Context) (list []Admin, total int64, err error) {
	return nil, 0, nil
}
