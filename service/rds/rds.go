package rds

import (
	"context"
	"go-build-admin/conf"

	"github.com/go-redis/redis/extra/redisotel"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// NewRedis .
func NewRedis(config *conf.Configuration, gLog *zap.Logger) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Redis.Host + ":" + config.Redis.Port,
		Password: config.Redis.Password,
		DB:       config.Redis.DB,
	})

	client.AddHook(redisotel.TracingHook{})
	if err := client.Ping(context.Background()).Err(); err != nil {
		gLog.Error("redis connect failed, err:", zap.Any("err", err))
		panic("failed to connect redis")
	}
	return client
}
