package model

import (
	"go-build-admin/app/pkg/random"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// User 会员表
type User struct {
	ID            int32     `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`      // ID
	GroupID       int32     `gorm:"column:group_id;not null;comment:分组ID" json:"group_id"`             // 分组ID
	Username      string    `gorm:"column:username;not null;comment:用户名" json:"username"`              // 用户名
	Nickname      string    `gorm:"column:nickname;not null;comment:昵称" json:"nickname"`               // 昵称
	Email         string    `gorm:"column:email;not null;comment:邮箱" json:"email"`                     // 邮箱
	Mobile        string    `gorm:"column:mobile;not null;comment:手机" json:"mobile"`                   // 手机
	Avatar        string    `gorm:"column:avatar;not null;comment:头像" json:"avatar"`                   // 头像
	Gender        int32     `gorm:"column:gender;not null;comment:性别:0=未知,1=男,2=女" json:"gender"`      // 性别:0=未知,1=男,2=女
	Birthday      time.Time `gorm:"column:birthday;comment:生日" json:"birthday"`                        // 生日
	Money         int32     `gorm:"column:money;not null;comment:余额" json:"money"`                     // 余额
	Score         int32     `gorm:"column:score;not null;comment:积分" json:"score"`                     // 积分
	LastLoginTime int64     `gorm:"column:last_login_time;comment:上次登录时间" json:"last_login_time"`      // 上次登录时间
	LastLoginIP   string    `gorm:"column:last_login_ip;not null;comment:上次登录IP" json:"last_login_ip"` // 上次登录IP
	LoginFailure  int32     `gorm:"column:login_failure;not null;comment:登录失败次数" json:"login_failure"` // 登录失败次数
	JoinIP        string    `gorm:"column:join_ip;not null;comment:加入IP" json:"join_ip"`               // 加入IP
	JoinTime      int64     `gorm:"column:join_time;comment:加入时间" json:"join_time"`                    // 加入时间
	Motto         string    `gorm:"column:motto;not null;comment:签名" json:"motto"`                     // 签名
	Password      string    `gorm:"column:password;not null;comment:密码" json:"password"`               // 密码
	Salt          string    `gorm:"column:salt;not null;comment:密码盐" json:"salt"`                      // 密码盐
	Status        string    `gorm:"column:status;not null;comment:状态" json:"status"`                   // 状态
	UpdateTime    int64     `gorm:"autoCreateTime;column:update_time;comment:更新时间" json:"update_time"` // 更新时间
	CreateTime    int64     `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
}

type OutUser struct {
	User
	Birthday string `json:"birthday"`
	Money    string `json:"money"`
}

type UserModel struct {
	sqlDB  *gorm.DB
	config *conf.Configuration
}

func NewUserModel(sqlDB *gorm.DB, config *conf.Configuration) *UserModel {
	return &UserModel{
		sqlDB:  sqlDB,
		config: config,
	}
}

func (s *UserModel) GetOne(ctx *gin.Context, id int32) (User, error) {
	data := User{}
	err := s.sqlDB.Omit("password", "salt").Where("id=?", id).First(&data).Error
	return data, err
}

func (s *UserModel) IsExist(ctx *gin.Context, fieldName string, fieldValue any, id int32) (User, error) {
	var err error
	data := User{}
	if slices.Contains([]string{"username", "mobile", "email"}, fieldName) {
		err = s.sqlDB.
			Omit("password", "salt").
			Where(fieldName+"=?", fieldValue).
			Where("id<>?", id).
			First(&data).Error
	}
	return data, err
}

func (s *UserModel) GetOneByEmail(ctx *gin.Context, email string) (User, error) {
	data := User{}
	err := s.sqlDB.Omit("password", "salt").Where("email=?", email).First(&data).Error
	return data, err
}

func (s *UserModel) GetOneByMobile(ctx *gin.Context, mobile string) (User, error) {
	data := User{}
	err := s.sqlDB.Omit("password", "salt").Where("mobile=?", mobile).First(&data).Error
	return data, err
}

func (s *UserModel) ValidatePassword(ctx *gin.Context, id int32, oldPassword string) bool {
	user := User{}
	s.sqlDB.Where("id=?", id).First(&user)
	return user.Password == utils.EncryptPassword(oldPassword, user.Salt)
}

func (s *UserModel) ResetPassword(ctx *gin.Context, id int32, password string) error {
	salt := random.Build("alnum", 16)
	password = utils.EncryptPassword(password, salt)
	err := s.sqlDB.Model(&User{}).Where("id=?", id).Updates(map[string]any{
		"salt":     salt,
		"password": password,
	}).Error
	return err
}

func (s *UserModel) Update(ctx *gin.Context, id int32, data map[string]any) error {
	err := s.sqlDB.Model(&User{}).Where("id=?", id).Updates(data).Error
	return err
}
