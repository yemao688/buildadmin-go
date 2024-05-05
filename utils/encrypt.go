package utils

import (
	"crypto/md5"
	"encoding/hex"
)

func Md5(content, salt string) string {
	// 创建一个新的MD5散列对象
	hasher := md5.New()
	// 将待加密字符串写入散列对象
	hasher.Write([]byte(content + salt))
	// 通过Sum方法计算最终的MD5散列值，返回值为16个字节（128位）
	hashBytes := hasher.Sum(nil)
	// 将散列值转换为十六进制字符串，便于显示和比较
	md5Hex := hex.EncodeToString(hashBytes)
	return md5Hex
}
