package utils

import (
	"math/rand"
	"time"

	"github.com/google/uuid"
)

func GenerateUuid() string {
	u := uuid.New()
	return u.String()
}

func GenerateRandomString(t string, length int) string {
	var letterBytes string
	rand.Seed(time.Now().UnixNano()) // 初始化随机数生成器

	switch t {
	case "alpha":
		letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	case "alnum":
		letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	case "numeric":
		letterBytes = "0123456789"
	case "noZero":
		letterBytes = "123456789"
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
