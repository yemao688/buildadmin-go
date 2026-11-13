package repository

import (
	"context"
	"errors"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/requesttx"
	"buildadmin-go/internal/pkg/systemroot"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// accountColumns 是 GetByAccount 系列方法允许查询的账号列白名单。字段名
// 来自调用方的账号类型，绝不能直接拼进 SQL。
var accountColumns = map[string]string{
	"username": "username",
	"email":    "email",
	"mobile":   "mobile",
}

// UserRepository 集中会员 user 表的读写；api 渠道的 GORM 只允许出现在
// repository 中，service/handler 一律经此包访问 user 表。
type UserRepository struct {
	db         *gorm.DB
	adminTable string
}

// NewUserRepository 构造会员仓库；adminTable 由配置前缀推出，供解析系统
// 根管理员归属使用。
func NewUserRepository(db *gorm.DB, config *conf.Configuration) *UserRepository {
	adminTable := ""
	if config != nil {
		adminTable = config.Database.Prefix + "admin"
	}
	return &UserRepository{db: db, adminTable: adminTable}
}

// Transaction 在事务中执行 fn。api 渠道当前没有请求事务中间件
// （requesttx.Bind 只在 admin 渠道接入），这里仍走 requesttx 的统一原语，
// 由 WithDB 提供无请求事务时的回退事务，与 persistence.BaseModel 语义一致。
// 对外签名保持无 context：会员 refresh 流程由 token.RefreshExecutor 接口
// 驱动，该接口不携带请求上下文。
func (r *UserRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return requesttx.Transaction(requesttx.WithDB(context.Background(), r.db), fn)
}

// GetByID 按主键取会员；记录不存在时返回 (nil, nil)，调用方必须先判 nil
// 再解引用。
func (r *UserRepository) GetByID(id int32) (*model.User, error) {
	var user model.User
	if err := r.db.Where("id=?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

// GetByAccount 按登录账号列（mobile/email/username）查询会员；字段不在白
// 名单时返回 invalid account field。无匹配行时返回 (nil, nil)（Scan 语义，
// 不产生 RecordNotFound 错误）。
func (r *UserRepository) GetByAccount(field, value string) (*model.User, error) {
	return r.getByAccount(r.db, field, value)
}

// GetByAccountIn 是 GetByAccount 的事务内版本，用于在同一事务里完成注册
// 唯一性校验与写入。
func (r *UserRepository) GetByAccountIn(tx *gorm.DB, field, value string) (*model.User, error) {
	return r.getByAccount(tx, field, value)
}

func (r *UserRepository) getByAccount(db *gorm.DB, field, value string) (*model.User, error) {
	column, ok := accountColumns[field]
	if !ok {
		return nil, cErr.BadRequest("invalid account field")
	}
	var user model.User
	if err := db.Model(&model.User{}).Where(column+"=?", value).Scan(&user).Error; err != nil {
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

// Create 在给定事务内写入新会员记录；唯一键冲突原样返回，由调用方经
// IsDuplicateKeyError 判定后映射业务错误。
func (r *UserRepository) Create(tx *gorm.DB, user *model.User) error {
	return tx.Create(user).Error
}

// ResolveRootAdminID 解析系统根管理员 id，作为新注册会员的默认归属；未配置
// 管理员时返回错误，由 service 映射为对外文案。
func (r *UserRepository) ResolveRootAdminID() (int32, error) {
	return systemroot.Resolver{DB: r.db, AdminTable: r.adminTable}.Resolve()
}

// IsDuplicateKeyError 判定写入错误是否为唯一键冲突（MySQL 1062 或 GORM 归一
// 化错误），供 service 把并发注册映射为业务错误。
func IsDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
