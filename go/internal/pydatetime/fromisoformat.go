// Package pydatetime implements the bounded datetime.fromisoformat subset used
// at Memento's JSON boundaries, without depending on service error envelopes.
package pydatetime

import (
	"encoding/json"
	"errors"
	"math"
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

// ParseError retains a stable broad category for service error mapping while
// exposing a parser reason to strict executor validation.
type ParseError struct {
	Kind   error
	Reason string
}

func (e *ParseError) Error() string              { return e.Kind.Error() + ": " + e.Reason }
func (e *ParseError) Unwrap() error              { return e.Kind }
func parseError(kind error, reason string) error { return &ParseError{Kind: kind, Reason: reason} }
func Reason(err error) string {
	var parsed *ParseError
	if errors.As(err, &parsed) {
		return parsed.Reason
	}
	return ""
}

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

// ParseJSON applies pydantic-core's JSON datetime input rule. Strict mode
// accepts strings only; lax mode also accepts finite JSON numbers, interpreting
// magnitudes beyond 20 billion as milliseconds rather than seconds.
func ParseJSON(value any, strict bool) (time.Time, error) {
	if text, ok := value.(string); ok {
		if number, err := strconv.ParseFloat(text, 64); err == nil && !math.IsInf(number, 0) && !math.IsNaN(number) {
			return unixNumber(number)
		}
		// pydantic-core's JSON grammar is deliberately narrower than
		// datetime.fromisoformat: no basic/week dates, arbitrary separators,
		// hour-only clocks or second/fractional timezone offsets.
		if reason := pydanticDiagnostic(text, strict); reason != "" {
			return time.Time{}, parseError(ErrInvalid, reason)
		}
		if strings.HasSuffix(text, "z") {
			text = text[:len(text)-1] + "Z"
		}
		return ParseAware(text)
	}
	if strict {
		return time.Time{}, parseError(ErrInvalid, "Input should be a valid datetime")
	}
	number, ok := value.(json.Number)
	if !ok {
		return time.Time{}, parseError(ErrInvalid, "Input should be a valid datetime")
	}
	parsed, err := number.Float64()
	if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
		return time.Time{}, parseError(ErrInvalid, "Input should be a valid datetime")
	}
	return unixNumber(parsed)
}
func pydanticDiagnostic(text string, strict bool) string {
	prefix := "Input should be a valid datetime"
	if !strict {
		prefix += " or date"
	}
	if len(text) < 4 {
		return prefix + ", input is too short"
	}
	if len(text) >= 4 && text[:4] == "0000" {
		return "Input should be a valid datetime, year 0 is out of range"
	}
	if len(text) < 10 {
		return prefix + ", input is too short"
	}
	if text[4] != '-' {
		return prefix + ", invalid date separator, expected `-`"
	}
	if text[5] < '0' || text[5] > '9' {
		return prefix + ", invalid character in month"
	}
	if text[7] != '-' {
		return prefix + ", invalid date separator, expected `-`"
	}
	if dayOutOfRange(text[:10]) {
		return prefix + ", day value is outside expected range"
	}
	if !strict {
		return prefix + ", unexpected extra characters at the end of the input"
	}
	if len(text) == 10 || !strings.ContainsRune("Tt _", rune(text[10])) {
		return prefix + ", invalid datetime separator, expected `T`, `t`, `_` or space"
	}
	if len(text) < 16 {
		return prefix + ", input is too short"
	}
	if text[13] != ':' {
		return prefix + ", invalid time separator, expected `:`"
	}
	hour, _ := strconv.Atoi(text[11:13])
	minute, _ := strconv.Atoi(text[14:16])
	if hour > 23 {
		return prefix + ", hour value is outside expected range of 0-23"
	}
	if minute > 59 {
		return prefix + ", minute value is outside expected range of 0-59"
	}
	position := 16
	if position < len(text) && text[position] == ':' {
		if position+3 > len(text) {
			return prefix + ", input is too short"
		}
		second, _ := strconv.Atoi(text[position+1 : position+3])
		if second > 59 {
			return prefix + ", second value is outside expected range of 0-59"
		}
		position += 3
		if position < len(text) && (text[position] == '.' || text[position] == ',') {
			position++
			for position < len(text) && text[position] >= '0' && text[position] <= '9' {
				position++
			}
		}
	}
	if position >= len(text) {
		return prefix + ", invalid timezone sign"
	}
	if text[position] == 'Z' || text[position] == 'z' {
		if position+1 == len(text) {
			return ""
		}
		return prefix + ", unexpected extra characters at the end of the input"
	}
	if text[position] != '+' && text[position] != '-' {
		return prefix + ", invalid timezone sign"
	}
	zone := text[position+1:]
	if len(zone) < 2 || zone[0] < '0' || zone[0] > '9' {
		return prefix + ", invalid timezone hour"
	}
	zhour, _ := strconv.Atoi(zone[:2])
	if len(zone) > 5 && zone[2] != ':' {
		return prefix + ", unexpected extra characters at the end of the input"
	}
	if len(zone) < 5 || zone[2] != ':' {
		return prefix + ", invalid timezone minute"
	}
	if zone[3] < '0' || zone[3] > '9' {
		return prefix + ", invalid timezone minute"
	}
	zminute, _ := strconv.Atoi(zone[3:5])
	if zminute > 59 {
		return prefix + ", timezone minute value is outside expected range of 0-59"
	}
	if zhour >= 24 {
		return prefix + ", timezone offset must be less than 24 hours"
	}
	if len(zone) > 5 {
		return prefix + ", unexpected extra characters at the end of the input"
	}
	return ""
}
func dayOutOfRange(date string) bool {
	year, _ := strconv.Atoi(date[:4])
	month, _ := strconv.Atoi(date[5:7])
	day, _ := strconv.Atoi(date[8:10])
	if month < 1 || month > 12 || day < 1 {
		return true
	}
	base := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return base.Month() != time.Month(month)
}

func unixNumber(number float64) (time.Time, error) {
	if math.Abs(number) > 2e10 {
		number /= 1000
	}
	seconds, fraction := math.Modf(number)
	micro := math.Round(fraction * 1e6)
	stamp := time.Unix(int64(seconds), int64(micro)*1000).UTC()
	if stamp.Year() < 1 || stamp.Year() > 9999 {
		return time.Time{}, ErrOverflow
	}
	return stamp, nil
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
