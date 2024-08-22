package utils

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"strings"
)

func EncryptPassword(content, salt string) string {
	hasher := md5.New()
	hasher.Write([]byte(content))
	md5Hex := hex.EncodeToString(hasher.Sum(nil)) + salt

	hasher.Reset()
	hasher.Write([]byte(md5Hex))
	md5Hex = hex.EncodeToString(hasher.Sum(nil))
	return md5Hex
}

func Md5(content string) string {
	// 创建一个新的MD5散列对象
	hasher := md5.New()
	// 将待加密字符串写入散列对象
	hasher.Write([]byte(content))
	// 通过Sum方法计算最终的MD5散列值，返回值为16个字节（128位）
	hashBytes := hasher.Sum(nil)
	// 将散列值转换为十六进制字符串，便于显示和比较
	md5Hex := hex.EncodeToString(hashBytes)
	return md5Hex
}

func MaskPhone(content string) string {
	matched, _ := regexp.MatchString("^1[3-9]\\d{9}$", content)
	// 根据匹配结果进行处理
	if matched {
		maskedUsername := strings.Replace(content, content[3:7], "****", 1)
		return maskedUsername

	}
	return content
}
