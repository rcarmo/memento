package execute

import (
	"encoding/json"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

func planBool(value any) (bool, string) {
	switch v := value.(type) {
	case bool:
		return v, ""
	case string:
		switch strings.ToLower(v) {
		case "1", "true", "t", "yes", "y", "on":
			return true, ""
		case "0", "false", "f", "no", "n", "off":
			return false, ""
		}
		return false, "Input should be a valid boolean, unable to interpret input"
	case json.Number:
		n, err := v.Float64()
		if err == nil && n == 1 {
			return true, ""
		}
		if err == nil && n == 0 {
			return false, ""
		}
		if err != nil || n != math.Trunc(n) {
			return false, "Input should be a valid boolean"
		}
		return false, "Input should be a valid boolean, unable to interpret input"
	default:
		return false, "Input should be a valid boolean"
	}
}

var planIntString = regexp.MustCompile(`^[+-]?[0-9](?:_?[0-9])*(?:\.0+)?$`)

func planInteger(value any) (json.Number, string) {
	switch v := value.(type) {
	case bool:
		if v {
			return "1", ""
		}
		return "0", ""
	case json.Number:
		if !strings.ContainsAny(string(v), ".eE") {
			n, ok := new(big.Int).SetString(string(v), 10)
			if ok {
				return json.Number(n.String()), ""
			}
			return "", "Input should be a valid integer"
		}
		n, err := v.Float64()
		if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
			return "", "Input should be a finite number"
		}
		if n != math.Trunc(n) {
			return "", "Input should be a valid integer, got a number with a fractional part"
		}
		if n >= math.Exp2(63) || n <= -math.Exp2(63) {
			return "", "Unable to parse input string as an integer, exceeded maximum size"
		}
		return json.Number(strconv.FormatInt(int64(n), 10)), ""
	case string:
		text := strings.TrimSpace(v)
		if !planIntString.MatchString(text) {
			return "", "Input should be a valid integer, unable to parse string as an integer"
		}
		text = strings.ReplaceAll(strings.SplitN(text, ".", 2)[0], "_", "")
		if len(strings.TrimLeft(strings.TrimLeft(text, "+-"), "0")) > 4300 {
			return "", "Unable to parse input string as an integer, exceeded maximum size"
		}
		n, _ := new(big.Int).SetString(text, 10)
		return json.Number(n.String()), ""
	default:
		return "", "Input should be a valid integer"
	}
}
func planTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case json.Number:
		n, err := v.Float64()
		return err != nil || n != 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true
	}
}

type Limits struct {
	MaxOperations    int         `json:"max_operations"`
	MaxIntermediates int         `json:"max_intermediates"`
	MaxRecords       int         `json:"max_records"`
	MaxOutputBytes   json.Number `json:"max_output_bytes"`
	MaxTimeSeconds   float64     `json:"max_time_seconds"`
}

func ParseLimits(value any) (Limits, error) {
	limits := Limits{12, 12, 50, "65536", 3}
	raw, ok := value.(map[string]any)
	if !ok {
		return Limits{}, &ValidationError{[]ValidationIssue{{"", "Input should be a valid dictionary or instance of ExecuteLimits"}}}
	}
	issues := []ValidationIssue{}
	for _, field := range []struct {
		name     string
		min, max int
		target   *int
	}{{"max_operations", 1, 32, &limits.MaxOperations}, {"max_intermediates", 1, 64, &limits.MaxIntermediates}, {"max_records", 1, 500, &limits.MaxRecords}, {"max_output_bytes", 512, 0, nil}} {
		value, exists := raw[field.name]
		if !exists {
			continue
		}
		number, message := planInteger(value)
		if message != "" {
			addIssue(&issues, field.name, message)
			continue
		}
		n, _ := new(big.Int).SetString(string(number), 10)
		if n.Cmp(big.NewInt(int64(field.min))) < 0 {
			addIssue(&issues, field.name, "Input should be greater than or equal to "+strconv.Itoa(field.min))
			continue
		}
		if field.max > 0 && n.Cmp(big.NewInt(int64(field.max))) > 0 {
			addIssue(&issues, field.name, "Input should be less than or equal to "+strconv.Itoa(field.max))
			continue
		}
		if field.target != nil {
			*field.target = int(n.Int64())
		} else {
			limits.MaxOutputBytes = number
		}
	}
	if value, exists := raw["max_time_seconds"]; exists {
		n, message := planFloat(value)
		if message != "" {
			addIssue(&issues, "max_time_seconds", message)
		} else if !(n <= 30) {
			addIssue(&issues, "max_time_seconds", "Input should be less than or equal to 30")
		} else if !(n > 0) {
			addIssue(&issues, "max_time_seconds", "Input should be greater than 0")
		} else {
			limits.MaxTimeSeconds = n
		}
	}
	extraFields(raw, []string{"max_operations", "max_intermediates", "max_records", "max_output_bytes", "max_time_seconds"}, "", &issues)
	if len(issues) > 0 {
		return Limits{}, &ValidationError{issues}
	}
	return limits, nil
}
func planFloat(value any) (float64, string) {
	switch v := value.(type) {
	case bool:
		if v {
			return 1, ""
		}
		return 0, ""
	case json.Number:
		n, err := v.Float64()
		if err != nil && !math.IsInf(n, 0) {
			return 0, "Input should be a valid number"
		}
		return n, ""
	case string:
		text := strings.TrimSpace(v)
		if strings.ContainsAny(text, "xXpP") {
			return 0, "Input should be a valid number, unable to parse string as a number"
		}
		n, err := strconv.ParseFloat(text, 64)
		if err != nil && !math.IsInf(n, 0) {
			return 0, "Input should be a valid number, unable to parse string as a number"
		}
		return n, ""
	default:
		return 0, "Input should be a valid number"
	}
}
