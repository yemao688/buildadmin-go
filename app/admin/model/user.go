package model

import (
	"errors"
	"fmt"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/random"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

const TableNameUser = "ba_user"

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
	Group         UserGroup `gorm:"foreignKey:GroupID" json:"group"`
}

func (*User) TableName() string {
	return TableNameUser
}

type SimpleUser struct {
	ID       int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`
	Username string `gorm:"column:username;not null;comment:用户名" json:"username"`
	Nickname string `gorm:"column:nickname;not null;comment:昵称" json:"nickname"`
}

func (*SimpleUser) TableName() string {
	return TableNameUser
}

type OutUser struct {
	User
	Birthday string `json:"birthday"`
	Money    string `json:"money"`
}

type UserModel struct {
	BaseModel
	config *conf.Configuration
}

func NewUserModel(sqlDB *gorm.DB, config *conf.Configuration) *UserModel {
	return &UserModel{
		BaseModel: BaseModel{
			TableName:        TableNameUser,
			Key:              "id",
			QuickSearchField: "username,nickname",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
		config: config,
	}
}

func (s *UserModel) DealData(ctx *gin.Context, data *User) (*OutUser, error) {
	outUser := OutUser{}
	if err := copier.Copy(&outUser, data); err != nil {
		return nil, err
	}
	outUser.Avatar = utils.DefaultUrl(data.Avatar, s.config.App.DefaultAvatar)
	outUser.Money = fmt.Sprintf("%.2f", float64(data.Money)/100)
	outUser.Birthday = data.Birthday.Format("2006-01-02")
	return &outUser, nil
}

func (s *UserModel) GetOne(ctx *gin.Context, id int32) (User, error) {
	data := User{}
	err := s.sqlDB.Table(s.TableName).Omit("password,salt").Where("id=?", id).First(&data).Error
	return data, err
}

func (s *UserModel) List(ctx *gin.Context) ([]*OutUser, int64, error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	var total int64 = 0
	list := []*User{}

	db := s.sqlDB.Model(&User{}).Joins("Group").Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Omit("password,salt").Order(orderS).Limit(limit).Offset(offset).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	result := []*OutUser{}
	for _, v := range list {
		outUser, err := s.DealData(ctx, v)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, outUser)
	}
	return result, total, nil
}

func (s *UserModel) Add(ctx *gin.Context, user User) error {
	//判断是否有重名的账号
	if err := s.sqlDB.Table(s.TableName).Where("username=?", user.Username).Take(&User{}).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		return cErr.BadRequest("Account not exist")
	}

	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Table(s.TableName).Omit("login_failure, last_login_time, last_login_ip").Create(&user).Error; err != nil {
		tx.Rollback()
		return err

	}
	return tx.Commit().Error
}

func (s *UserModel) Edit(ctx *gin.Context, user User) error {
	//判断是否有重名的账号
	if err := s.sqlDB.Table(s.TableName).Where("id<>? and username=?", user.ID, user.Username).Take(&User{}).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		return cErr.BadRequest("Account not exist")
	}

	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Table(s.TableName).Omit("password, salt, login_failure, last_login_time").Save(&user).Error; err != nil {
		tx.Rollback()
		return err

	}
	return tx.Commit().Error
}

func (s *UserModel) ResetPassword(ctx *gin.Context, id int32, password string) error {
	salt := random.Build("alnum", 16)
	password = utils.EncryptPassword(password, salt)
	err := s.sqlDB.Table(s.TableName).Where("id=?", id).Updates(map[string]interface{}{
		"salt":     salt,
		"password": password,
	}).Error
	return err
}

func (s *UserModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}
