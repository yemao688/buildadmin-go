package utils

import "time"

const SysTimeform = "2006-01-02 15:04:05"
const SysTimeformShort = "2006-01-02"

var SysTimeLocation, _ = time.LoadLocation("Asia/Shanghai")

// 当前时间的时间戳
func NowUnix() int64 {
	return time.Now().In(SysTimeLocation).Unix()
}

// 将unix时间戳格式化为yyyymmdd H:i:s格式字符串
func FormatFromUnixTime(t int64) string {
	if t > 0 {
		return time.Unix(t, 0).In(SysTimeLocation).Format(SysTimeform)
	} else {
		return time.Now().In(SysTimeLocation).Format(SysTimeform)
	}
}

// 将unix时间戳格式化为yyyymmdd格式字符串
func FormatFromUnixTimeShort(t int64) string {
	if t > 0 {
		return time.Unix(t, 0).In(SysTimeLocation).Format(SysTimeformShort)
	} else {
		return time.Now().In(SysTimeLocation).Format(SysTimeformShort)
	}
}

// 将字符串转成时间
func ParseTime(str string) (time.Time, error) {
	return time.ParseInLocation(SysTimeform, str, SysTimeLocation)
}

func ParseTimeShort(str string) (time.Time, error) {
	return time.ParseInLocation(SysTimeformShort, str, SysTimeLocation)
}

func ParseTimeFormat(format string, str string) (time.Time, error) {
	return time.ParseInLocation(format, str, SysTimeLocation)
}

// 得到当前时间到下一天零点的延时
func NextDayDuration() time.Duration {
	year, month, day := time.Now().In(SysTimeLocation).Add(time.Hour * 24).Date()
	next := time.Date(year, month, day, 0, 0, 0, 0, SysTimeLocation)
	return next.Sub(time.Now().In(SysTimeLocation))
}

// 获取今日开始时间戳
func TodayStartUnix() int64 {
	year, month, day := time.Now().In(SysTimeLocation).Date()
	next := time.Date(year, month, day, 0, 0, 0, 0, SysTimeLocation)
	return next.Unix()
}

// 获取今日结束时间戳
func TodayEndUnix() int64 {
	year, month, day := time.Now().In(SysTimeLocation).Date()
	next := time.Date(year, month, day, 23, 59, 59, 0, SysTimeLocation)
	return next.Unix()
}

func DayStartUnix(t time.Time) int64 {
	year, month, day := t.In(SysTimeLocation).Date()
	next := time.Date(year, month, day, 0, 0, 0, 0, SysTimeLocation)
	return next.Unix()
}

func DayEndUnix(t time.Time) int64 {
	year, month, day := t.In(SysTimeLocation).Date()
	next := time.Date(year, month, day, 23, 59, 59, 0, SysTimeLocation)
	return next.Unix()
}

//获取周一时间戳
func GetWeekStart(t time.Time) time.Time {
	offset := time.Monday - t.Weekday()
	if offset > 0 {
		offset -= 6
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, int(offset))
}

//获取本周日
func GetWeekEnd(t time.Time) time.Time {
	return GetWeekStart(t).AddDate(0, 0, 7)
}

//格式化 UNIX 时间戳为人易读的字符串 TODO:
func HumanTime(t time.Time) string {
	return ""
}
