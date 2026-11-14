// Package random 生成随机串与随机数。
package random

import (
	"crypto/md5" //nolint:gosec // 输出格式与 PHP 上游 md5 helper 保持一致（32 位十六进制），不可改用其它摘要
	cryptorand "crypto/rand"
	"crypto/sha1" //nolint:gosec // 输出格式与 PHP 上游 sha1 helper 保持一致（40 位十六进制），不可改用其它摘要
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SecureIntn 返回 [0,n) 区间内的密码学安全随机整数；n<=0 时 panic，与
// math/rand.Intn 的契约一致。包内与验证码/点击验证码共用的安全随机入口。
func SecureIntn(n int) int {
	if n <= 0 {
		panic("random: invalid argument to SecureIntn")
	}
	v, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic("random: crypto/rand unavailable: " + err.Error())
	}
	return int(v.Int64())
}

// secureInt63 返回非负的密码学安全 63 位随机整数。
func secureInt63() int64 {
	var buf [8]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		panic("random: crypto/rand unavailable: " + err.Error())
	}
	return int64(binary.BigEndian.Uint64(buf[:]) & math.MaxInt64)
}

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
		randomNum := SecureIntn(1000000)
		randomNumStr := strconv.Itoa(randomNum)
		// 获取当前时间戳并添加随机数以增加唯一性
		currentTime := time.Now().Format("2006-01-02 15:04:05")
		uniqueID := currentTime + randomNumStr
		// 计算MD5散列
		hasher := md5.New() //nolint:gosec // 输出格式与 PHP 上游 md5 helper 兼容（32 位十六进制），随机源已改用 crypto/rand
		hasher.Write([]byte(uniqueID))
		md5Bytes := hasher.Sum(nil)
		return fmt.Sprintf("%x", md5Bytes)
	case "encrypt":
	case "sha1":
		// 生成随机数并转换为字符串
		randomNum := secureInt63()
		randomNumStr := strconv.FormatInt(randomNum, 10)
		// 获取当前时间戳，并附加随机数和额外的唯一性（true 对应的在PHP中是添加微秒）
		currentTime := strconv.FormatInt(time.Now().UnixNano(), 10)
		uniqueID := currentTime + randomNumStr
		// 计算SHA1散列
		hasher := sha1.New() //nolint:gosec // 输出格式与 PHP 上游 sha1 helper 兼容（40 位十六进制），随机源已改用 crypto/rand
		hasher.Write([]byte(uniqueID))
		sha1Bytes := hasher.Sum(nil)
		// 将SHA1散列转换为16进制字符串
		return fmt.Sprintf("%x", sha1Bytes)
	}
	return t
}

func shuffleString(s string) string {
	runes := []rune(s)
	for i := len(runes) - 1; i > 0; i-- {
		j := SecureIntn(i + 1)
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
