package token

import (
	"context"
	"encoding/json"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/conf"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/unknwon/com"
)

type RedisDriver struct {
	config conf.Token
	rdb    *redis.Client
}

func NewRedisDriver(rdb *redis.Client, config conf.Token) *RedisDriver {
	return &RedisDriver{rdb: rdb, config: config}
}

func (d RedisDriver) Set(token string, t string, user_id int32, expire int64) error {
	if expire < 0 {
		expire = d.config.Expire
	}

	if expire != 0 {
		expire = time.Now().Unix() + expire
	}

	encryptToken, err := GetEncryptedToken(token, d.config.Algo, d.config.Key)
	if err != nil {
		return err
	}

	data := Token{
		Token:      encryptToken,
		Type:       t,
		UserID:     user_id,
		CreateTime: time.Now().Unix(),
		ExpireTime: expire,
	}

	ctx := context.Background()
	dataBytes, _ := json.Marshal(data)
	if expire > 0 {
		if t == "admin" || t == "user" {
			// 增加 redis中的 token 过期时间，以免 token 过期自动刷新永远无法触发
			expire = expire * 2
		}
		d.rdb.SetEX(ctx, encryptToken, dataBytes, time.Second*time.Duration(expire))
	} else {
		d.rdb.Set(ctx, encryptToken, dataBytes, 0)
	}
	userKey := d.GetUserKey(user_id)
	d.rdb.SAdd(ctx, userKey, encryptToken)
	return nil
}

func (d RedisDriver) Get(token string, expirationException bool) (*Token, error) {
	encryptToken, err := GetEncryptedToken(token, d.config.Algo, d.config.Key)
	if err != nil {
		return nil, err
	}
	dataStr, err := d.rdb.Get(context.Background(), encryptToken).Result()
	if err != nil {
		return nil, err
	}

	var data Token
	err = json.Unmarshal([]byte(dataStr), &data)
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

func (d RedisDriver) Check(token string, t string, user_id int32, expirationException bool) bool {
	data, err := d.Get(token, expirationException)
	if err != nil {
		return false
	}
	if !expirationException && data.ExpireTime > 0 && data.ExpireTime < time.Now().Unix() {
		return false
	}
	return data.Type == t && data.UserID == user_id
}
func (d RedisDriver) Delete(token string) error {
	data, err := d.Get(token, false)
	if err != nil {
		return err
	}

	encryptToken, err := GetEncryptedToken(token, d.config.Algo, d.config.Key)
	if err != nil {
		return err
	}
	d.rdb.Del(context.Background(), encryptToken)
	d.rdb.SRem(context.Background(), d.GetUserKey(data.UserID), encryptToken)
	return nil
}

func (d RedisDriver) Clear(t string, user_id int32) error {
	members, _ := d.rdb.SMembers(context.Background(), d.GetUserKey(user_id)).Result()
	d.rdb.Del(context.Background(), d.GetUserKey(user_id))
	d.rdb.Del(context.Background(), members...)
	return nil
}

func (d RedisDriver) GetUserKey(user_id int32) string {
	return "up:" + com.ToStr(user_id)
}
