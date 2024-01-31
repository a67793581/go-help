package date

import (
	"fmt"
	"time"
)

func Now2YMD() string {
	return time.Now().Format("2006-01-02")
}
func Day2YMD(day time.Time) string {
	return day.Format("2006-01-02")
}

func ToDay() time.Time {
	resultTime := time.Now()
	resultTime = time.Date(resultTime.Year(), resultTime.Month(), resultTime.Day(), 0, 0, 0, 0, resultTime.Location())
	return resultTime
}
func Yesterday() time.Time {
	return ToDay().AddDate(0, 0, -1)
}
func Tomorrow() time.Time {
	return ToDay().AddDate(0, 0, +1)
}
func NextWeekMonday() time.Time {
	// 获取今天0点的时间
	return NextWeekMondayByTime(ToDay())
}
func NextWeekMondayByTime(currentTime time.Time) time.Time {
	// 计算下一周周一的时间
	daysUntilNextMonday := NextWeekMondayByTimeGetDays(currentTime)
	return currentTime.Add(time.Duration(daysUntilNextMonday) * 24 * time.Hour)
}
func NextWeekMondayByTimeGetDays(currentTime time.Time) int {
	// 获取今天是本周的第几天（0表示周日，1表示周一，依此类推）
	currentDay := int(currentTime.Weekday())

	// 计算下一周周一的时间
	daysUntilNextMonday := (7 - currentDay + 1) % 7 // 计算距离下一周周一还有多少天
	if daysUntilNextMonday == 0 {
		daysUntilNextMonday = 7
	}
	return daysUntilNextMonday
}

func WeekMonday() time.Time {
	return NextWeekMonday().Add(-7 * 24 * time.Hour)
}

func Now2Week() string {
	year, week := time.Now().ISOWeek()
	return fmt.Sprintf("%v_%v", year, week)
}

const (
	Second = 1
	Minute = Second * 60
	Hour   = Minute * 60

	Day   = Hour * 24
	Week  = Day * 7
	Month = Day * 30
)
