package user

import (
	"go-build-admin/app/common/model"
	"go-build-admin/app/pkg/random"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"slices"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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

func (s *UserModel) GetOne(ctx *gin.Context, id int32) (model.User, error) {
	data := model.User{}
	err := s.sqlDB.Omit("password", "salt").Where("id=?", id).First(&data).Error
	return data, err
}

func (s *UserModel) IsExist(ctx *gin.Context, fieldName string, fieldValue any, id int32) (model.User, error) {
	var err error
	data := model.User{}
	if slices.Contains([]string{"username", "mobile", "email"}, fieldName) {
		err = s.sqlDB.
			Omit("password", "salt").
			Where(fieldName+"=?", fieldValue).
			Where("id<>?", id).
			First(&data).Error
	}
	return data, err
}

func (s *UserModel) GetOneByEmail(ctx *gin.Context, email string) (model.User, error) {
	data := model.User{}
	err := s.sqlDB.Omit("password", "salt").Where("email=?", email).First(&data).Error
	return data, err
}

func (s *UserModel) GetOneByMobile(ctx *gin.Context, mobile string) (model.User, error) {
	data := model.User{}
	err := s.sqlDB.Omit("password", "salt").Where("mobile=?", mobile).First(&data).Error
	return data, err
}

func (s *UserModel) ValidatePassword(ctx *gin.Context, id int32, oldPassword string) bool {
	user := model.User{}
	s.sqlDB.Where("id=?", id).First(&user)
	return user.Password == utils.EncryptPassword(oldPassword, user.Salt)
}

func (s *UserModel) ResetPassword(ctx *gin.Context, id int32, password string) error {
	salt := random.Build("alnum", 16)
	password = utils.EncryptPassword(password, salt)
	err := s.sqlDB.Model(&model.User{}).Where("id=?", id).Updates(map[string]any{
		"salt":     salt,
		"password": password,
	}).Error
	return err
}

func (s *UserModel) Update(ctx *gin.Context, id int32, data map[string]any) error {
	err := s.sqlDB.Model(&model.User{}).Where("id=?", id).Updates(data).Error
	return err
}
