package token

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go-build-admin/conf"
	"hash"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Driver interface {
	Set(token string, t string, user_id int32, expire int64) error
	Get(token string, expirationException bool) (*Token, error)
	Check(token string, t string, user_id int32, expirationException bool) bool
	Delete(token string) error
	Clear(t string, user_id int32) error
}

type Token struct {
	Token      string `gorm:"column:token;primaryKey;comment:Token" json:"token"`  // Token
	Type       string `gorm:"column:type;not null;comment:类型" json:"type"`         // 类型
	UserID     int32  `gorm:"column:user_id;not null;comment:用户ID" json:"user_id"` // 用户ID
	CreateTime int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`  // 创建时间
	ExpireTime int64  `gorm:"column:expire_time;comment:过期时间" json:"expire_time"`  // 过期时间
	ExpiresIn  int64  `json:"expires_in"`                                          // 返回剩余有效时间
}

type TokenHelper struct {
	Driver
}

func NewTokenHelper(config *conf.Configuration, log *zap.Logger, sqlDB *gorm.DB, rdb *redis.Client) *TokenHelper {
	//通过配置判断
	var driver Driver
	if config.Token.Default == "redis" {
		driver = NewMysqlDriver(sqlDB, config)
	} else {
		driver = NewRedisDriver(rdb, config)
	}
	return &TokenHelper{Driver: driver}
}

func (h TokenHelper) Set(token string, t string, user_id int32, expire int64) error {

	return h.Driver.Set(token, t, user_id, expire)
}
func (h TokenHelper) Get(token string, expirationException bool) (*Token, error) {
	return h.Driver.Get(token, expirationException)
}
func (h TokenHelper) Check(token string, t string, user_id int32, expirationException bool) bool {
	return h.Driver.Check(token, t, user_id, expirationException)
}
func (h TokenHelper) Delete(token string) error {
	return h.Driver.Delete(token)
}
func (h TokenHelper) Clear(t string, user_id int32) error {
	return h.Driver.Clear(t, user_id)
}

func GetEncryptedToken(token string, algo string, k string) (string, error) {
	key := []byte(k)
	var h func() hash.Hash
	switch algo {
	case "sha256":
		h = sha256.New
	case "sha1":
		h = sha1.New
	case "md5":
		h = md5.New
	default:
		return "", fmt.Errorf("unsupported hashing algorithm: %s", algo)
	}

	mac := hmac.New(h, key)
	_, err := mac.Write([]byte(token))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func GetExpiredIn(expireTime int64) int64 {
	if expireTime != 0 {
		if n := expireTime - time.Now().Unix(); n > 0 {
			return n
		}
		return 0
	}
	return 365 * 86400
}
