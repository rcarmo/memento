// Package pydatetime implements the bounded datetime.fromisoformat subset used
// at Memento's JSON boundaries, without depending on service error envelopes.
package pydatetime

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid ISO 8601 datetime")
	ErrNaive    = errors.New("datetime is not timezone-aware")
	ErrOverflow = errors.New("date value out of range")
)

var week = regexp.MustCompile(`^([0-9]{4})(-?)W([0-9]{2})(?:-?([0-9]))?`)
var dateTime = regexp.MustCompile(`(?s)^([0-9]{4})-?([0-9]{2})-?([0-9]{2})(?:.(.*))?$`)
var clockParts = regexp.MustCompile(`^([0-9]{2})(?::?([0-9]{2}))?(?::?([0-9]{2}))?(?:[.,]([0-9]+))?$`)

// ParseAware normalises an aware Python ISO datetime to UTC and microseconds.
// time.Time is accepted for internal callers; JSON callers pass strings.
func ParseAware(value any) (time.Time, error) {
	if stamp, ok := value.(time.Time); ok {
		return stamp.UTC().Truncate(time.Microsecond), nil
	}
	text, ok := value.(string)
	if !ok {
		return time.Time{}, ErrNaive
	}
	text = strings.ReplaceAll(text, "Z", "+00:00")
	if match := week.FindStringSubmatch(text); match != nil {
		year, _ := strconv.Atoi(match[1])
		number, _ := strconv.Atoi(match[3])
		day := 1
		if match[4] != "" {
			day, _ = strconv.Atoi(match[4])
		}
		extended := match[2] == "-"
		if year < 1 || number < 1 || number > 53 || day < 1 || day > 7 || match[4] != "" && (extended && len(match[0]) != 10 || !extended && len(match[0]) != 8) {
			return time.Time{}, ErrInvalid
		}
		jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
		weekday := (int(jan4.Weekday()) + 6) % 7
		date := jan4.AddDate(0, 0, -weekday+(number-1)*7+day-1)
		isoYear, isoWeek := date.ISOWeek()
		if isoYear != year || isoWeek != number {
			return time.Time{}, ErrInvalid
		}
		text = date.Format("2006-01-02") + text[len(match[0]):]
	}
	match := dateTime.FindStringSubmatch(text)
	if match == nil {
		return time.Time{}, ErrInvalid
	}
	date := text[:8]
	if len(text) > 4 && text[4] == '-' {
		if len(text) < 10 || text[7] != '-' {
			return time.Time{}, ErrInvalid
		}
		date = text[:10]
	} else if strings.Contains(date, "-") {
		return time.Time{}, ErrInvalid
	}
	year, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	day, _ := strconv.Atoi(match[3])
	base := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if year < 1 || base.Year() != year || int(base.Month()) != month || base.Day() != day {
		return time.Time{}, ErrInvalid
	}
	clock := match[4]
	offset, aware := 0, false
	if zoneAt := strings.IndexAny(clock, "+-"); zoneAt >= 0 {
		zone := clock[zoneAt+1:]
		parts := clockParts.FindStringSubmatch(zone)
		if parts == nil || !clockShape(zone) {
			return time.Time{}, ErrInvalid
		}
		h, min, sec, micro := values(parts)
		if h == 0 && min == 0 && sec == 0 {
			micro = 0
		}
		offset = (h*3600+min*60+sec)*1000000 + micro
		if offset >= 24*3600*1000000 {
			return time.Time{}, ErrInvalid
		}
		if clock[zoneAt] == '-' {
			offset = -offset
		}
		aware = true
		clock = clock[:zoneAt]
	}
	hour, minute, second, micro := 0, 0, 0, 0
	if clock != "" {
		parts := clockParts.FindStringSubmatch(clock)
		if parts == nil || !clockShape(clock) {
			return time.Time{}, ErrInvalid
		}
		hour, minute, second, micro = values(parts)
		if hour > 23 || minute > 59 || second > 59 {
			return time.Time{}, ErrInvalid
		}
	} else if len(text) != len(date) {
		return time.Time{}, ErrInvalid
	}
	if !aware {
		return time.Time{}, ErrNaive
	}
	result := base.Add(time.Duration(hour*3600+minute*60+second)*time.Second + time.Duration(micro)*time.Microsecond - time.Duration(offset)*time.Microsecond)
	if result.Year() < 1 || result.Year() > 9999 {
		return time.Time{}, ErrOverflow
	}
	return result, nil
}

func clockShape(text string) bool {
	whole := strings.SplitN(strings.ReplaceAll(text, ",", "."), ".", 2)[0]
	return !strings.Contains(whole, ":") || len(whole) == 5 && whole[2] == ':' || len(whole) == 8 && whole[2] == ':' && whole[5] == ':'
}
func values(parts []string) (int, int, int, int) {
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
