package util

import "time"

const SysTimeform = "2006-01-02 15:04:05"
const SysTimeformShort = "2006-01-02"

var SysTimeLocation, _ = time.LoadLocation("Asia/Shanghai")

// 将unix时间戳格式化为yyyymmdd H:i:s格式字符串
func FormatFromUnixTime(t int64) string {
	if t > 0 {
		return time.Unix(t, 0).In(SysTimeLocation).Format(SysTimeform)
	} else {
		return time.Now().In(SysTimeLocation).Format(SysTimeform)
	}
}

// 将字符串转成时间
func ParseTime(str string) (time.Time, error) {
	return time.ParseInLocation(SysTimeform, str, SysTimeLocation)
}

func ParseTimeShort(str string) (time.Time, error) {
	return time.ParseInLocation(SysTimeformShort, str, SysTimeLocation)
}
