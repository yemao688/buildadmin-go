package repository

import (
	"buildadmin-go/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserRepository 集中会员 user 表的读写；api 渠道的 GORM 只允许出现在
// repository 与领域服务（member）中，service/handler 一律经此包访问 user 表。
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// DB 暴露底层连接，供领域服务执行非 user 表查询（如 systemroot 解析 admin 表）。
func (r *UserRepository) DB() *gorm.DB {
	return r.db
}

// Transaction 在事务中执行 fn（与既有 sqlDB.Transaction 语义一致，不参与
// 请求事务链）。
func (r *UserRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// IsEnabled 仅读取 status 列判断会员是否启用。
func (r *UserRepository) IsEnabled(id int32) bool {
	var user model.User
	err := r.db.Model(&model.User{}).Select("status").Where("id=?", id).First(&user).Error
	return err == nil && user.Status == "enable"
}

// GetByID 按主键取会员；记录不存在时返回 (nil, nil)。
func (r *UserRepository) GetByID(id int32) (*model.User, error) {
	var user model.User
	if err := r.db.Where("id=?", id).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// LockByID 在给定事务内按主键加行锁（FOR UPDATE）取会员；错误原样返回。
func (r *UserRepository) LockByID(tx *gorm.DB, id int32) (*model.User, error) {
	var user model.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByAccount 按登录账号列（mobile/email/username）查询会员；
// 无匹配行时返回 (nil, nil)（Scan 语义，不产生 RecordNotFound 错误）。
func (r *UserRepository) GetByAccount(field, value string) (*model.User, error) {
	var user model.User
	if err := r.db.Model(&model.User{}).Where(field+"=?", value).Scan(&user).Error; err != nil {
		return nil, err
	}
	if user.ID == 0 {
		return nil, nil
	}
	return &user, nil
}

// UpdateLoginMeta 更新登录元信息（失败次数/最后登录时间/最后登录 IP）。
func (r *UserRepository) UpdateLoginMeta(id int32, failure int32, loginTime int64, ip string) error {
	return r.db.Model(&model.User{}).Where("id=?", id).Updates(map[string]any{
		"login_failure":   failure,
		"last_login_time": loginTime,
		"last_login_ip":   ip,
	}).Error
}

// ResetLoginFailure 仅清零登录失败次数（登录重试窗口到期后）。
func (r *UserRepository) ResetLoginFailure(id int32) error {
	return r.db.Model(&model.User{}).Where("id=?", id).Updates(map[string]any{
		"login_failure": 0,
	}).Error
}

// Create 写入新会员记录。
func (r *UserRepository) Create(user *model.User) error {
	return r.db.Create(user).Error
}
