package utils

import (
	"fmt"
	"time"
)

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

// 获取周一时间戳
func GetWeekStart(t time.Time) time.Time {
	offset := time.Monday - t.Weekday()
	if offset > 0 {
		offset -= 6
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, int(offset))
}

// 获取本周日
func GetWeekEnd(t time.Time) time.Time {
	return GetWeekStart(t).AddDate(0, 0, 7)
}

// 格式化 UNIX 时间戳成易读的字符串
func HumanTime(t time.Time, lang string) string {
	timeDiff := t.Unix() - NowUnix()

	// 定义时间间隔和单位
	chunks := []struct {
		Seconds int64
		Name    string
	}{
		{60 * 60 * 24 * 365, "year"},
		{60 * 60 * 24 * 30, "month"},
		{60 * 60 * 24 * 7, "week"},
		{60 * 60 * 24, "day"},
		{60 * 60, "hour"},
		{60, "minute"},
		{1, "second"},
	}

	// 计算时间差
	count := 0
	name := ""
	for _, chunk := range chunks {
		count = int(timeDiff / chunk.Seconds)
		if count != 0 {
			break
		}
		name = chunk.Name
	}

	pluralize := ""
	if count > 1 {
		pluralize = "s"
	}

	if lang == "" {
		return fmt.Sprintf("%d %s%s ago", count, name, pluralize)
	}

	zhMap := map[string]string{
		"second": "秒前",
		"minute": "分钟前",
		"hour":   "小时前",
		"day":    "天前",
		"week":   "周前",
		"month":  "月前",
		"year":   "年前",
	}
	return fmt.Sprintf("%d %s", count, zhMap[name])
}
