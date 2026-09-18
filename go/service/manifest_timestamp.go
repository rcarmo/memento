package service

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rcarmo/memento/go/umcp"
)

// Calendar ISO forms accepted by datetime.fromisoformat, including basic date/
// time, arbitrary one-code-point separator, comma fractions and second offsets.
// Week dates are converted through the ISO week containing 4 January.
var manifestWeek = regexp.MustCompile(`^([0-9]{4})(-?)W([0-9]{2})(?:-?([0-9]))?`)
var manifestDateTime = regexp.MustCompile(`(?s)^([0-9]{4})-?([0-9]{2})-?([0-9]{2})(?:.(.*))?$`)
var manifestClockParts = regexp.MustCompile(`^([0-9]{2})(?::?([0-9]{2}))?(?::?([0-9]{2}))?(?:[.,]([0-9]+))?$`)

func manifestTimestamp(value any) (time.Time, error) {
	invalid := func() (time.Time, error) {
		return time.Time{}, manifestInvalid("manifest timestamps must be ISO 8601 values")
	}
	if stamp, ok := value.(time.Time); ok {
		return stamp.UTC().Truncate(time.Microsecond), nil
	}
	text, ok := value.(string)
	if !ok {
		return time.Time{}, manifestInvalid("manifest timestamps must be timezone-aware")
	}
	text = strings.ReplaceAll(text, "Z", "+00:00")
	if week := manifestWeek.FindStringSubmatch(text); week != nil {
		year, _ := strconv.Atoi(week[1])
		number, _ := strconv.Atoi(week[3])
		day := 1
		if week[4] != "" {
			day, _ = strconv.Atoi(week[4])
		}
		extended := week[2] == "-"
		if year < 1 || number < 1 || number > 53 || day < 1 || day > 7 || week[4] != "" && (extended && len(week[0]) != 10 || !extended && len(week[0]) != 8) {
			return invalid()
		}
		jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
		weekday := (int(jan4.Weekday()) + 6) % 7
		date := jan4.AddDate(0, 0, -weekday+(number-1)*7+day-1)
		isoYear, isoWeek := date.ISOWeek()
		if isoYear != year || isoWeek != number {
			return invalid()
		}
		text = date.Format("2006-01-02") + text[len(week[0]):]
	}
	m := manifestDateTime.FindStringSubmatch(text)
	if m == nil {
		return invalid()
	}
	date := text[:8]
	if len(text) > 4 && text[4] == '-' {
		if len(text) < 10 || text[7] != '-' {
			return invalid()
		}
		date = text[:10]
	} else if strings.Contains(date, "-") {
		return invalid()
	}
	year, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	day, _ := strconv.Atoi(m[3])
	base := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if year < 1 || base.Year() != year || int(base.Month()) != month || base.Day() != day {
		return invalid()
	}
	clock := m[4]
	offset := 0
	aware := false
	zoneAt := strings.IndexAny(clock, "+-")
	if zoneAt >= 0 {
		zone := clock[zoneAt+1:]
		parts := manifestClockParts.FindStringSubmatch(zone)
		if parts == nil || !manifestClockShape(zone) {
			return invalid()
		}
		h, min, s, micro := clockValues(parts)
		// CPython treats an all-zero whole-second offset as UTC, ignoring
		// its fractional part.
		if h == 0 && min == 0 && s == 0 {
			micro = 0
		}
		offset = (h*3600+min*60+s)*1000000 + micro
		// Python normalises offset minute/second overflow; only the total is bounded.
		if offset >= 24*3600*1000000 {
			return invalid()
		}
		if clock[zoneAt] == '-' {
			offset = -offset
		}
		aware = true
		clock = clock[:zoneAt]
	}
	hour, minute, second, micro := 0, 0, 0, 0
	if clock != "" {
		parts := manifestClockParts.FindStringSubmatch(clock)
		if parts == nil || !manifestClockShape(clock) {
			return invalid()
		}
		hour, minute, second, micro = clockValues(parts)
		if hour > 23 || minute > 59 || second > 59 {
			return invalid()
		}
	} else if len(text) != len(date) {
		return invalid()
	}
	if !aware {
		return time.Time{}, manifestInvalid("manifest timestamps must be timezone-aware")
	}
	result := base.Add(time.Duration(hour*3600+minute*60+second)*time.Second + time.Duration(micro)*time.Microsecond - time.Duration(offset)*time.Microsecond)
	if result.Year() < 1 || result.Year() > 9999 {
		return time.Time{}, umcp.ExecutionError{Type: "OverflowError", Message: "date value out of range"}
	}
	return result, nil
}
func manifestClockShape(text string) bool {
	whole := strings.SplitN(strings.ReplaceAll(text, ",", "."), ".", 2)[0]
	// fromisoformat forbids mixing basic and extended time components.
	return !strings.Contains(whole, ":") || len(whole) == 5 && whole[2] == ':' || len(whole) == 8 && whole[2] == ':' && whole[5] == ':'
}
func clockValues(parts []string) (int, int, int, int) {
	hour, _ := strconv.Atoi(parts[1])
	minute, _ := strconv.Atoi(parts[2])
	second, _ := strconv.Atoi(parts[3])
	fraction := parts[4]
	if len(fraction) > 6 {
		fraction = fraction[:6]
	}
	fraction += strings.Repeat("0", 6-len(fraction))
	micro, _ := strconv.Atoi(fraction)
	return hour, minute, second, micro
}
