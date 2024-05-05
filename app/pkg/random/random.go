package random

import (
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func Uuid() string {
	u := uuid.New()
	return u.String()
}

func Build(t string, length int) string {
	switch t {
	case "alpha":
		fallthrough
	case "alnum":
		fallthrough
	case "numeric":
		fallthrough
	case "noZero":
		poolMap := map[string]string{
			"alpha":   "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			"alnum":   "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			"numeric": "0123456789",
			"noZero":  "123456789",
		}
		pool := poolMap[t]
		repeatTimes := int(math.Ceil(float64(length) / float64(len(poolMap[t]))))
		repeatedPool := strings.Repeat(pool, repeatTimes)
		shuffledPool := shuffleString(repeatedPool)
		return shuffledPool[:length]
	case "unique":
	case "md5":
		rand.Seed(time.Now().UnixNano()) // 确保随机性
		randomNum := rand.Intn(1000000)
		randomNumStr := strconv.Itoa(randomNum)
		// 获取当前时间戳并添加随机数以增加唯一性
		currentTime := time.Now().Format("2006-01-02 15:04:05")
		uniqueID := currentTime + randomNumStr
		// 计算MD5散列
		hasher := md5.New()
		hasher.Write([]byte(uniqueID))
		md5Bytes := hasher.Sum(nil)
		return fmt.Sprintf("%x", md5Bytes)
	case "encrypt":
	case "sha1":
		// 生成随机数并转换为字符串
		rand.Seed(time.Now().UnixNano()) // 确保随机性
		randomNum := rand.Int63()
		randomNumStr := strconv.FormatInt(randomNum, 10)
		// 获取当前时间戳，并附加随机数和额外的唯一性（true 对应的在PHP中是添加微秒）
		currentTime := strconv.FormatInt(time.Now().UnixNano(), 10)
		uniqueID := currentTime + randomNumStr
		// 计算SHA1散列
		hasher := sha1.New()
		hasher.Write([]byte(uniqueID))
		sha1Bytes := hasher.Sum(nil)
		// 将SHA1散列转换为16进制字符串
		return fmt.Sprintf("%x", sha1Bytes)
	}
	return t
}

func shuffleString(s string) string {
	rand.Seed(time.Now().UnixNano())
	runes := []rune(s)
	for i := len(runes) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
