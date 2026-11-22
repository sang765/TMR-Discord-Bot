package store

import (
	"sort"
	"strconv"
	"time"
)

// dateToOrdinal chuyển "2006-01-02" thành số ngày kể từ epoch (UTC),
// để so sánh liên tiếp không dính tháng/năm nhảy.
func dateToOrdinal(s string) (int, bool) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0, false
	}
	return int(t.Unix() / 86400), true
}

func ordinal(today string) int {
	n, _ := dateToOrdinal(today)
	return n
}

func sortInts(a []int) { sort.Ints(a) }

// VNDay trả về chuỗi "2006-01-02" theo giờ VN (UTC+7) của thời điểm t.
func VNDay(t time.Time) string {
	return t.In(time.FixedZone("+07:00", 7*3600)).Format("2006-01-02")
}

// ParseSnowflakeTime chuyển Discord snowflake (dạng string) sang time.Time.
// Discord epoch = 2015-01-01T00:00:00Z = 1420070400000 ms; timestamp nằm ở 22 bit cao.
func ParseSnowflakeTime(id string) (time.Time, bool) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}, false
	}
	ms := (n >> 22) + 1420070400000
	return time.UnixMilli(ms).UTC(), true
}

// SnowflakeBefore trả về snowflake của mốc t (dùng làm tham số `before`).
func SnowflakeBefore(t time.Time) string {
	ms := t.UnixMilli() - 1420070400000
	if ms < 0 {
		ms = 0
	}
	return strconv.FormatInt(ms<<22, 10)
}
