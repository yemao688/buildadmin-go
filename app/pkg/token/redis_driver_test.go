package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
)

type redisStoreFake struct {
	values map[string]string
	sets   map[string]map[string]struct{}
	ttls   map[string]time.Duration

	setEXErr   error
	setErr     error
	saddErr    error
	getErr     error
	membersErr error
	delErr     error
	sremErr    error
}

func newRedisStoreFake() *redisStoreFake {
	return &redisStoreFake{
		values: make(map[string]string),
		sets:   make(map[string]map[string]struct{}),
		ttls:   make(map[string]time.Duration),
	}
}

func (f *redisStoreFake) SetEX(_ context.Context, key string, value interface{}, expiration time.Duration) (string, error) {
	if f.setEXErr != nil {
		return "", f.setEXErr
	}
	f.values[key] = stringValue(value)
	f.ttls[key] = expiration
	return "OK", nil
}

func (f *redisStoreFake) Set(_ context.Context, key string, value interface{}, expiration time.Duration) (string, error) {
	if f.setErr != nil {
		return "", f.setErr
	}
	f.values[key] = stringValue(value)
	f.ttls[key] = expiration
	return "OK", nil
}

func (f *redisStoreFake) SAdd(_ context.Context, key string, members ...interface{}) (int64, error) {
	if f.saddErr != nil {
		return 0, f.saddErr
	}
	if f.sets[key] == nil {
		f.sets[key] = make(map[string]struct{})
	}
	var added int64
	for _, member := range members {
		value := stringValue(member)
		if _, exists := f.sets[key][value]; !exists {
			added++
		}
		f.sets[key][value] = struct{}{}
	}
	return added, nil
}

func (f *redisStoreFake) Get(_ context.Context, key string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	value, ok := f.values[key]
	if !ok {
		return "", redis.Nil
	}
	return value, nil
}

func (f *redisStoreFake) SMembers(_ context.Context, key string) ([]string, error) {
	if f.membersErr != nil {
		return nil, f.membersErr
	}
	var members []string
	for member := range f.sets[key] {
		members = append(members, member)
	}
	return members, nil
}

func (f *redisStoreFake) Del(_ context.Context, keys ...string) (int64, error) {
	if f.delErr != nil {
		return 0, f.delErr
	}
	var deleted int64
	for _, key := range keys {
		if _, exists := f.values[key]; exists {
			delete(f.values, key)
			deleted++
		}
	}
	return deleted, nil
}

func (f *redisStoreFake) SRem(_ context.Context, key string, members ...interface{}) (int64, error) {
	if f.sremErr != nil {
		return 0, f.sremErr
	}
	var removed int64
	for _, member := range members {
		value := stringValue(member)
		if _, exists := f.sets[key][value]; exists {
			delete(f.sets[key], value)
			removed++
		}
	}
	return removed, nil
}

func stringValue(value interface{}) string {
	switch value := value.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return fmt.Sprint(value)
	}
}

func newFakeRedisDriver(store *redisStoreFake) RedisDriver {
	config := &conf.Configuration{}
	config.Token.Algo = "sha256"
	config.Token.Key = "redis-driver-test-key"
	return RedisDriver{config: config, store: store}
}

func redisTokenValue(t *testing.T, rawToken string, tokenType string, userID int32) (string, string) {
	t.Helper()
	encrypted, err := GetEncryptedToken(rawToken, "sha256", "redis-driver-test-key")
	require.NoError(t, err)
	data, err := json.Marshal(Token{Token: encrypted, Type: tokenType, UserID: userID})
	require.NoError(t, err)
	return encrypted, string(data)
}

func TestRedisDriverSetUsesDurationTTLAndTypedIndex(t *testing.T) {
	store := newRedisStoreFake()
	driver := newFakeRedisDriver(store)

	started := time.Now().Unix()
	require.NoError(t, driver.Set("admin-token", "admin", 7, 45))
	encrypted, _ := redisTokenValue(t, "admin-token", "admin", 7)

	require.Equal(t, 90*time.Second, store.ttls[encrypted])
	require.Contains(t, store.sets[driver.GetUserKeyFor("admin", 7)], encrypted)
	require.NotContains(t, store.sets[driver.GetUserKey(7)], encrypted)
	rawData := store.values[encrypted]
	var stored Token
	require.NoError(t, json.Unmarshal([]byte(rawData), &stored))
	require.InDelta(t, started+45, stored.ExpireTime, 1)
}

func TestRedisDriverSetReturnsRedisErrors(t *testing.T) {
	setErr := errors.New("set failed")
	store := newRedisStoreFake()
	store.setEXErr = setErr
	driver := newFakeRedisDriver(store)
	require.ErrorIs(t, driver.Set("token", "admin-refresh", 1, 45), setErr)

	saddErr := errors.New("index failed")
	store = newRedisStoreFake()
	store.saddErr = saddErr
	driver = newFakeRedisDriver(store)
	require.ErrorIs(t, driver.Set("token", "admin-refresh", 1, 45), saddErr)
}

func TestRedisDriverClearKeepsOtherTokenTypes(t *testing.T) {
	store := newRedisStoreFake()
	driver := newFakeRedisDriver(store)
	require.NoError(t, driver.Set("admin-token", "admin", 7, 0))
	require.NoError(t, driver.Set("admin-refresh", "admin-refresh", 7, 0))
	require.NoError(t, driver.Set("user-token", "user", 7, 0))

	require.NoError(t, driver.Clear("admin", 7))
	require.False(t, driver.Check("admin-token", "admin", 7))
	require.True(t, driver.Check("admin-refresh", "admin-refresh", 7))
	require.True(t, driver.Check("user-token", "user", 7))
}

func TestRedisDriverClearReadsAndCleansLegacyIndexByActualType(t *testing.T) {
	store := newRedisStoreFake()
	driver := newFakeRedisDriver(store)
	adminEncrypted, adminData := redisTokenValue(t, "legacy-admin", "admin", 9)
	userEncrypted, userData := redisTokenValue(t, "legacy-user", "user", 9)
	store.values[adminEncrypted] = adminData
	store.values[userEncrypted] = userData
	store.sets[driver.GetUserKey(9)] = map[string]struct{}{
		adminEncrypted: {},
		userEncrypted:  {},
	}

	require.NoError(t, driver.Clear("admin", 9))
	require.NotContains(t, store.values, adminEncrypted)
	require.Contains(t, store.values, userEncrypted)
	require.NotContains(t, store.sets[driver.GetUserKey(9)], adminEncrypted)
	require.Contains(t, store.sets[driver.GetUserKey(9)], userEncrypted)
}

func TestRedisDriverPropagatesGetAndClearErrors(t *testing.T) {
	getErr := errors.New("get failed")
	store := newRedisStoreFake()
	store.getErr = getErr
	driver := newFakeRedisDriver(store)
	_, err := driver.Get("token")
	require.ErrorIs(t, err, getErr)

	membersErr := errors.New("members failed")
	store = newRedisStoreFake()
	store.membersErr = membersErr
	driver = newFakeRedisDriver(store)
	require.ErrorIs(t, driver.Clear("admin", 1), membersErr)
}

func TestTokenHelperGetForWorksWithRedisDriver(t *testing.T) {
	store := newRedisStoreFake()
	driver := newFakeRedisDriver(store)
	require.NoError(t, driver.Set("admin-token", "admin", 7, 0))
	helper := &TokenHelper{Driver: driver}

	data, err := helper.GetFor("admin-token", "admin")
	require.NoError(t, err)
	require.Equal(t, int32(7), data.UserID)
	_, err = helper.GetFor("admin-token", "user")
	require.EqualError(t, err, `token type mismatch: expected "user", got "admin"`)
}
