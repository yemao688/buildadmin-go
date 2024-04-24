package token

import (
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/conf"
	"time"

	"github.com/gin-contrib/cache/persistence"
	"gorm.io/gorm"
)

type MysqlDriver struct {
	sqlDB  *gorm.DB
	config conf.Token
}

func NewMysqlDriver(sqlDB *gorm.DB, config conf.Token) *MysqlDriver {
	return &MysqlDriver{sqlDB: sqlDB, config: config}
}

func (d MysqlDriver) Set(token string, t string, user_id int32, expire int64) error {
	if expire < 0 {
		expire = d.config.Expire
	}

	if expire != 0 {
		expire = time.Now().Unix() + expire
	}

	token, err := GetEncryptedToken(token, d.config.Algo, d.config.Key)
	if err != nil {
		return err
	}

	err = d.sqlDB.Create(&Token{
		Token:      token,
		Type:       t,
		UserID:     user_id,
		CreateTime: time.Now().Unix(),
		ExpireTime: expire,
	}).Error
	if err != nil {
		return err
	}

	store := persistence.NewInMemoryStore(time.Minute)
	var lastCacheCleanupTime int64
	var stamp = time.Now().Unix()
	err = store.Get("last_cache_cleanup_time", lastCacheCleanupTime)
	if err != nil || lastCacheCleanupTime < stamp-172800 {
		store.Set("", stamp, 172800)
		d.sqlDB.Where("expire_time < ? AND expire_time > 0 ", stamp).Delete(&Token{})
	}
	return nil
}

func (d MysqlDriver) Get(token string, expirationException bool) (*Token, error) {
	encryptToken, err := GetEncryptedToken(token, d.config.Algo, d.config.Key)
	if err != nil {
		return nil, err
	}
	var data Token
	err = d.sqlDB.Where("token = ? ", encryptToken).First(&data).Error
	if err != nil {
		return nil, err
	}
	// 返回未加密的token给客户端使用
	data.Token = token
	// 返回剩余有效时间
	data.ExpiresIn = GetExpiredIn(data.ExpireTime)
	if data.ExpireTime > 0 && data.ExpireTime < time.Now().Unix() && expirationException {
		// token过期-触发前端刷新token
		return nil, cErr.Unauthorized("Token expiration:", 409)
	}
	return &data, nil
}

func (d MysqlDriver) Check(token string, t string, user_id int32, expirationException bool) bool {
	data, err := d.Get(token, expirationException)
	if err != nil {
		return false
	}
	if !expirationException && data.ExpireTime > 0 && data.ExpireTime < time.Now().Unix() {
		return false
	}
	return data.Type == t && data.UserID == user_id
}

func (d MysqlDriver) Delete(token string) error {
	token, err := GetEncryptedToken(token, d.config.Algo, d.config.Key)
	if err != nil {
		return err
	}
	d.sqlDB.Where("token = ? ", token).Delete(&Token{})
	return nil
}

func (d MysqlDriver) Clear(t string, user_id int32) error {
	d.sqlDB.Where("type = ? AND user_id = ? ", t, user_id).Delete(&Token{})
	return nil
}
