package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/conf"
)

type failIndexRemoveOnceHook struct {
	userKey string
	mu      sync.Mutex
	failed  bool
}

func (h *failIndexRemoveOnceHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if cmd.Name() != "srem" {
		return ctx, nil
	}
	args := cmd.Args()
	if len(args) != 3 || args[1] != h.userKey {
		return ctx, nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.failed {
		return ctx, nil
	}
	h.failed = true
	return ctx, errors.New("injected user index remove failure")
}

func (h *failIndexRemoveOnceHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (h *failIndexRemoveOnceHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *failIndexRemoveOnceHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func TestRedisDriverClearRetriesStaleIndexAfterIndexRemoveFailure(t *testing.T) {
	miniRedis, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(miniRedis.Close)

	client := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	driver := NewRedisDriver(client, newMysqlDriverTestConfig())

	const userID int32 = 42
	require.NoError(t, driver.Set("retry-token", "user", userID, 3600))
	encryptedToken, err := GetEncryptedToken("retry-token", driver.config.Token.Algo, driver.config.Token.Key)
	require.NoError(t, err)
	userKey := driver.GetUserKeyFor("user", userID)
	client.AddHook(&failIndexRemoveOnceHook{userKey: userKey})

	require.Error(t, driver.Clear("user", userID))
	require.Equal(t, int64(0), client.Exists(context.Background(), encryptedToken).Val())
	require.Equal(t, int64(1), client.Exists(context.Background(), userKey).Val())

	require.NoError(t, driver.Clear("user", userID))
	require.Equal(t, int64(0), client.Exists(context.Background(), userKey).Val())
}

func TestRedisDriverClearScopesTokensByType(t *testing.T) {
	miniRedis, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(miniRedis.Close)
	client := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	driver := NewRedisDriver(client, newMysqlDriverTestConfig())

	const userID int32 = 7
	require.NoError(t, driver.Set("user-token", "user", userID, 3600))
	require.NoError(t, driver.Set("admin-token", "admin", userID, 3600))
	require.NoError(t, driver.Set("refresh-token", "user-refresh", userID, 3600))
	encryptedUserToken, err := GetEncryptedToken("user-token", driver.config.Token.Algo, driver.config.Token.Key)
	require.NoError(t, err)
	require.True(t, client.SIsMember(context.Background(), driver.GetUserKeyFor("user", userID), encryptedUserToken).Val())
	require.Equal(t, int64(0), client.Exists(context.Background(), driver.GetUserKey(userID)).Val())

	require.NoError(t, driver.Clear("user", userID))
	require.False(t, driver.Check("user-token", "user", userID))
	require.True(t, driver.Check("admin-token", "admin", userID))
	require.True(t, driver.Check("refresh-token", "user-refresh", userID))
}

func TestRedisDriverClearTransitionsMixedLegacyIndexByType(t *testing.T) {
	miniRedis, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(miniRedis.Close)
	client := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	driver := NewRedisDriver(client, newMysqlDriverTestConfig())

	const userID int32 = 8
	for _, tokenType := range []string{"admin", "user", "user-refresh"} {
		require.NoError(t, driver.Set(tokenType+"-token", tokenType, userID, 3600))
	}
	legacyUserKey := driver.GetUserKey(userID)
	for _, tokenName := range []string{"admin-token", "user-token", "user-refresh-token"} {
		encryptedToken, err := GetEncryptedToken(tokenName, driver.config.Token.Algo, driver.config.Token.Key)
		require.NoError(t, err)
		require.NoError(t, client.SAdd(context.Background(), legacyUserKey, encryptedToken).Err())
	}

	require.NoError(t, driver.Clear("user", userID))
	require.False(t, driver.Check("user-token", "user", userID))
	require.True(t, driver.Check("admin-token", "admin", userID))
	require.True(t, driver.Check("user-refresh-token", "user-refresh", userID))
	legacyMembers, err := client.SMembers(context.Background(), legacyUserKey).Result()
	require.NoError(t, err)
	require.Len(t, legacyMembers, 2)
}

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

func (f *redisStoreFake) Eval(_ context.Context, _ string, keys []string, args ...interface{}) error {
	if f.setEXErr != nil {
		return f.setEXErr
	}
	if f.saddErr != nil {
		return f.saddErr
	}
	if len(keys) != 2 || len(args) < 2 {
		return errors.New("invalid eval arguments")
	}
	ttl := time.Duration(0)
	switch value := args[0].(type) {
	case int64:
		ttl = time.Duration(value) * time.Second
	case int:
		ttl = time.Duration(value) * time.Second
	case time.Duration:
		ttl = value
	case string:
		var seconds int64
		_, _ = fmt.Sscan(value, &seconds)
		ttl = time.Duration(seconds) * time.Second
	}
	f.values[keys[1]] = stringValue(args[1])
	f.ttls[keys[1]] = ttl
	if f.sets[keys[0]] == nil {
		f.sets[keys[0]] = make(map[string]struct{})
	}
	f.sets[keys[0]][keys[1]] = struct{}{}
	return nil
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

func TestRedisDriverKeyPrefix(t *testing.T) {
	store := newRedisStoreFake()
	driver := newFakeRedisDriver(store)

	// 向后兼容：prefix 为空时保持历史 "up:..." key 格式
	require.Equal(t, "up:7", driver.GetUserKey(7))
	require.Equal(t, "up:user:7", driver.GetUserKeyFor("user", 7))

	// 非空 prefix：所有索引 key 增加 "prefix:" 前缀
	driver.config.Redis.Prefix = "ba"
	require.Equal(t, "ba:up:7", driver.GetUserKey(7))
	require.Equal(t, "ba:up:user:7", driver.GetUserKeyFor("user", 7))
	require.Equal(t, "ba:up:admin:7", driver.GetUserKeyFor("admin-refresh", 7))

	// 端到端：Set/Clear 使用带前缀的索引，旧无前缀索引不受影响
	require.NoError(t, driver.Set("admin-token", "admin", 7, 0))
	encrypted, _ := redisTokenValue(t, "admin-token", "admin", 7)
	require.Contains(t, store.sets["ba:up:admin:7"], encrypted)
	require.NotContains(t, store.sets["up:admin:7"], encrypted)
	require.NotContains(t, store.sets["up:7"], encrypted)

	require.NoError(t, driver.Clear("admin", 7))
	require.False(t, driver.Check("admin-token", "admin", 7))
	require.NotContains(t, store.sets["ba:up:admin:7"], encrypted)
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
