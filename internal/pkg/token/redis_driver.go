package token

import (
	"context"
	"encoding/json"
	"errors"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/conf"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/unknwon/com"
)

type redisStore interface {
	SetEX(context.Context, string, interface{}, time.Duration) (string, error)
	Set(context.Context, string, interface{}, time.Duration) (string, error)
	SAdd(context.Context, string, ...interface{}) (int64, error)
	Get(context.Context, string) (string, error)
	SMembers(context.Context, string) ([]string, error)
	Del(context.Context, ...string) (int64, error)
	SRem(context.Context, string, ...interface{}) (int64, error)
	Eval(context.Context, string, []string, ...interface{}) error
}

type redisClientStore struct {
	client *redis.Client
}

func (s redisClientStore) SetEX(ctx context.Context, key string, value interface{}, expiration time.Duration) (string, error) {
	return s.client.SetEX(ctx, key, value, expiration).Result()
}

func (s redisClientStore) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) (string, error) {
	return s.client.Set(ctx, key, value, expiration).Result()
}

func (s redisClientStore) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return s.client.SAdd(ctx, key, members...).Result()
}

func (s redisClientStore) Get(ctx context.Context, key string) (string, error) {
	return s.client.Get(ctx, key).Result()
}

func (s redisClientStore) SMembers(ctx context.Context, key string) ([]string, error) {
	return s.client.SMembers(ctx, key).Result()
}

func (s redisClientStore) Del(ctx context.Context, keys ...string) (int64, error) {
	return s.client.Del(ctx, keys...).Result()
}

func (s redisClientStore) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return s.client.SRem(ctx, key, members...).Result()
}

func (s redisClientStore) Eval(ctx context.Context, script string, keys []string, args ...interface{}) error {
	return s.client.Eval(ctx, script, keys, args...).Err()
}

type RedisDriver struct {
	config *conf.Configuration
	rdb    *redis.Client
	store  redisStore
}

func NewRedisDriver(rdb *redis.Client, config *conf.Configuration) *RedisDriver {
	return &RedisDriver{rdb: rdb, store: redisClientStore{client: rdb}, config: config}
}

func (d RedisDriver) client() redisStore {
	if d.store != nil {
		return d.store
	}
	return redisClientStore{client: d.rdb}
}

func (d RedisDriver) Set(token string, t string, user_id int32, expire int64) error {
	expireTime := int64(0)
	if expire != 0 {
		expireTime = time.Now().Unix() + expire
	}

	encryptToken, err := GetEncryptedToken(token, d.config.Token.Algo, d.config.Token.Key)
	if err != nil {
		return err
	}

	data := Token{
		Token:      encryptToken,
		Type:       t,
		UserID:     user_id,
		CreateTime: time.Now().Unix(),
		ExpireTime: expireTime,
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	ctx := context.Background()
	store := d.client()
	ttl := expire
	if ttl > 0 && (t == "admin" || t == "user") {
		// Keep access tokens available after their logical expiry so Get can
		// return the same refreshable expiration error as the MySQL driver.
		ttl *= 2
	}
	const setTokenAndIndex = `
redis.call('SADD', KEYS[1], KEYS[2])
if tonumber(ARGV[1]) > 0 then
  return redis.call('SET', KEYS[2], ARGV[2], 'EX', ARGV[1])
end
return redis.call('SET', KEYS[2], ARGV[2])`
	if err := store.Eval(ctx, setTokenAndIndex, []string{d.GetUserKeyFor(t, user_id), encryptToken}, ttl, string(dataBytes)); err != nil {
		return err
	}
	return nil
}

func (d RedisDriver) Get(token string) (*Token, error) {
	encryptToken, err := GetEncryptedToken(token, d.config.Token.Algo, d.config.Token.Key)
	if err != nil {
		return nil, err
	}
	dataStr, err := d.client().Get(context.Background(), encryptToken)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, cErr.BadRequest("Please login first", 303)
		}
		return nil, err
	}

	var data Token
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return nil, err
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

func (d RedisDriver) Check(token string, t string, user_id int32) bool {
	data, err := d.Get(token)
	if err != nil {
		return false
	}
	if data.ExpireTime > 0 && data.ExpireTime < time.Now().Unix() {
		return false
	}
	return data.Type == t && data.UserID == user_id
}

func (d RedisDriver) Delete(token string) error {
	data, err := d.Get(token)
	if err != nil {
		return err
	}

	encryptToken, err := GetEncryptedToken(token, d.config.Token.Algo, d.config.Token.Key)
	if err != nil {
		return err
	}
	ctx := context.Background()
	store := d.client()
	if _, err := store.Del(ctx, encryptToken); err != nil {
		return err
	}
	return d.removeFromIndexes(ctx, store, data.Type, data.UserID, encryptToken)
}

func (d RedisDriver) Clear(t string, user_id int32) error {
	ctx := context.Background()
	store := d.client()
	keys := []string{d.GetUserKeyFor(t, user_id), d.GetUserKey(user_id)}
	members := make(map[string]struct{})
	for _, key := range keys {
		values, err := store.SMembers(ctx, key)
		if err != nil {
			return err
		}
		for _, member := range values {
			members[member] = struct{}{}
		}
	}

	for member := range members {
		data, err := d.getStoredToken(ctx, store, member)
		if errors.Is(err, redis.Nil) {
			if err := d.removeFromIndexes(ctx, store, t, user_id, member); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if data.UserID != user_id || data.Type != t {
			continue
		}
		if _, err := store.Del(ctx, member); err != nil {
			return err
		}
		if err := d.removeFromIndexes(ctx, store, data.Type, data.UserID, member); err != nil {
			return err
		}
	}
	return nil
}

func (d RedisDriver) getStoredToken(ctx context.Context, store redisStore, encryptedToken string) (*Token, error) {
	dataStr, err := store.Get(ctx, encryptedToken)
	if err != nil {
		return nil, err
	}
	var data Token
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (d RedisDriver) removeFromIndexes(ctx context.Context, store redisStore, tokenType string, userID int32, encryptedToken string) error {
	if _, err := store.SRem(ctx, d.GetUserKeyFor(tokenType, userID), encryptedToken); err != nil {
		return err
	}
	_, err := store.SRem(ctx, d.GetUserKey(userID), encryptedToken)
	return err
}

func (d RedisDriver) GetUserKey(user_id int32) string {
	return "up:" + com.ToStr(user_id)
}

func (d RedisDriver) GetUserKeyFor(t string, user_id int32) string {
	return "up:" + tokenIndexType(t) + ":" + com.ToStr(user_id)
}

func (d RedisDriver) GetTypeUserKey(t string, user_id int32) string {
	return d.GetUserKeyFor(t, user_id)
}

func tokenIndexType(t string) string {
	switch t {
	case "admin", "admin-refresh":
		return "admin"
	case "user", "user-refresh":
		return "user"
	default:
		return t
	}
}
