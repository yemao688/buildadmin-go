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
	config *conf.Configuration
}

func NewMysqlDriver(sqlDB *gorm.DB, config *conf.Configuration) *MysqlDriver {
	return &MysqlDriver{sqlDB: sqlDB, config: config}
}

func (d MysqlDriver) Set(token string, t string, user_id int32, expire int64) error {
	if expire != 0 {
		expire = time.Now().Unix() + expire
	}

	token, err := GetEncryptedToken(token, d.config.Token.Algo, d.config.Token.Key)
	if err != nil {
		return err
	}
	err = d.sqlDB.Table("ba_token").Create(&Token{
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
		d.sqlDB.Table("ba_token").Where("expire_time < ? AND expire_time > 0 ", stamp).Delete(&Token{})
	}
	return nil
}

func (d MysqlDriver) Get(token string) (*Token, error) {
	encryptToken, err := GetEncryptedToken(token, d.config.Token.Algo, d.config.Token.Key)
	if err != nil {
		return nil, err
	}
	var data Token
	err = d.sqlDB.Table("ba_token").Where("token = ? ", encryptToken).First(&data).Error
	if err != nil {
		return nil, cErr.BadRequest("Please login first", 303)
	}
	// 返回未加密的token给客户端使用
	data.Token = token
	// 返回剩余有效时间
	data.ExpiresIn = GetExpiredIn(data.ExpireTime)
	if data.ExpireTime > 0 && data.ExpireTime < time.Now().Unix() {
		// token过期-触发前端刷新token
		return nil, cErr.Unauthorized("Token expiration", 409)
	}
	return &data, nil
}

func (d MysqlDriver) Check(token string, t string, user_id int32) bool {
	data, err := d.Get(token)
	if err != nil {
		return false
	}
	if data.ExpireTime > 0 && data.ExpireTime < time.Now().Unix() {
		return false
	}
	return data.Type == t && data.UserID == user_id
}

func (d MysqlDriver) Delete(token string) error {
	token, err := GetEncryptedToken(token, d.config.Token.Algo, d.config.Token.Key)
	if err != nil {
		return err
	}
	d.sqlDB.Table("ba_token").Where("token = ? ", token).Delete(&Token{})
	return nil
}

func (d MysqlDriver) Clear(t string, user_id int32) error {
	d.sqlDB.Table("ba_token").Where("type = ? AND user_id = ? ", t, user_id).Delete(&Token{})
	return nil
}
