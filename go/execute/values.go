// Package execute ports the JSON-domain value kernel of MemoryExecutor. It is
// not yet a plan validator, operation runner or registered memory_execute tool.
package execute

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Error retains the source exception family for the runner's catch boundaries.
type Error struct{ Kind, Message string }

func (e *Error) Error() string        { return e.Message }
func valueError(message string) error { return &Error{"ValueError", message} }

var savePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,31}$`)
var segmentPattern = regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_]*|[0-9]+)$`)
var referencePattern = regexp.MustCompile(`^\$[A-Za-z][A-Za-z0-9_]{0,31}(?:\.(?:[A-Za-z_][A-Za-z0-9_]*|[0-9]+))*$`)

// Python re.match(...$) accepts a final newline; fullmatch does not.
func validSavedName(name string) bool { return savePattern.MatchString(strings.TrimSuffix(name, "\n")) }
func validSegment(segment string) bool {
	return segmentPattern.MatchString(strings.TrimSuffix(segment, "\n"))
}

// ContainsReferences validates every nested reference, even if an earlier
// sibling already contains one (Python eagerly constructs any([...])).
func ContainsReferences(value any) (bool, error) {
	found := false
	switch v := value.(type) {
	case string:
		if !strings.HasPrefix(v, "$") {
			return false, nil
		}
		if utf8.RuneCountInString(v) > 256 || !referencePattern.MatchString(v) {
			return false, valueError("invalid saved reference (use $name.field or $name.0.field)")
		}
		return true, nil
	case []any:
		for _, item := range v {
			has, err := ContainsReferences(item)
			if err != nil {
				return false, err
			}
			found = found || has
		}
	case map[string]any:
		for _, item := range v {
			has, err := ContainsReferences(item)
			if err != nil {
				return false, err
			}
			found = found || has
		}
	}
	return found, nil
}

// ResolveReferences substitutes complete strings beginning with $, never
// interpolating embedded references or resolving object keys. Inputs are not
// mutated; resolved saved values themselves retain reference identity.
func ResolveReferences(value any, saved map[string]any) (any, error) {
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "$") {
			return ResolveReference(v, saved)
		}
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			resolved, err := ResolveReferences(item, saved)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil
	case map[string]any:
		result := map[string]any{}
		for key, item := range v {
			resolved, err := ResolveReferences(item, saved)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil
	}
	return value, nil
}
func ResolveReference(reference string, saved map[string]any) (any, error) {
	if !strings.HasPrefix(reference, "$") {
		return reference, nil
	}
	parts := strings.Split(reference[1:], ".")
	if !validSavedName(parts[0]) {
		return nil, valueError("invalid reference: " + reference)
	}
	current, ok := saved[parts[0]]
	if !ok {
		return nil, valueError("unknown reference: " + reference)
	}
	for _, segment := range parts[1:] {
		if !validSegment(segment) {
			return nil, valueError("invalid reference segment: " + reference)
		}
		if items, ok := current.([]any); ok {
			index, err := referenceIndex(segment)
			if err != nil {
				return nil, err
			}
			if index.Cmp(big.NewInt(int64(len(items)))) >= 0 {
				return nil, valueError("reference index out of range: " + reference)
			}
			current = items[index.Int64()]
			continue
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, valueError("reference does not resolve to a container: " + reference)
		}
		current, ok = object[segment]
		if !ok {
			return nil, valueError("reference field not found: " + reference)
		}
	}
	return current, nil
}
func referenceIndex(segment string) (*big.Int, error) {
	index, ok := new(big.Int).SetString(strings.TrimSpace(segment), 10)
	if !ok {
		return nil, valueError("invalid literal for int() with base 10: '" + strings.ReplaceAll(segment, "\n", `\n`) + "'")
	}
	return index, nil
}
func ExtractField(value any, path string) (any, error) {
	current := value
	for _, segment := range strings.Split(path, ".") {
		if !validSegment(segment) {
			return nil, valueError("invalid field path: " + path)
		}
		switch v := current.(type) {
		case []any:
			index, err := referenceIndex(segment)
			if err != nil {
				return nil, err
			}
			if !index.IsInt64() || strconv.IntSize == 32 && index.BitLen() > 31 {
				return nil, &Error{"IndexError", "cannot fit 'int' into an index-sized integer"}
			}
			if index.Int64() >= int64(len(v)) {
				return nil, &Error{"IndexError", "list index out of range"}
			}
			current = v[index.Int64()]
		case map[string]any:
			var ok bool
			current, ok = v[segment]
			if !ok {
				return nil, valueError("field not found: " + path)
			}
		default:
			return nil, valueError("field not found: " + path)
		}
	}
	return current, nil
}

// BoundValue recursively caps every list, not object member count or string
// length. Lists under asset operations are exempted by the caller/runner.
func BoundValue(value any, maxRecords int) any {
	switch v := value.(type) {
	case []any:
		out := []any{}
		for _, item := range v[:sliceEnd(len(v), maxRecords)] {
			out = append(out, BoundValue(item, maxRecords))
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			out[key] = BoundValue(item, maxRecords)
		}
		return out
	default:
		return value
	}
}
func sliceEnd(length, limit int) int {
	if limit < 0 {
		return max(0, length+limit)
	}
	return min(length, limit)
}

type SavedOperation struct {
	Operation string
	SaveAs    *string
}
type ReturnProjection struct {
	Name   *string
	Ref    string
	Fields []string
	Limit  *int
}

// ProjectReturns preserves Python's asset exemptions, failed-save suppression,
// list-wrapping for fields, fallback naming, and last duplicate name wins.
func ProjectReturns(operations []SavedOperation, returns []ReturnProjection, saved map[string]any, last any, failed map[string]bool, maxRecords int) (map[string]any, error) {
	if len(returns) == 0 {
		return map[string]any{"result": last}, nil
	}
	result := map[string]any{}
	for _, projection := range returns {
		root := strings.SplitN(strings.TrimPrefix(projection.Ref, "$"), ".", 2)[0]
		if failed[root] {
			continue
		}
		value, err := ResolveReference(projection.Ref, saved)
		if err != nil {
			return nil, err
		}
		asset := false
		for _, op := range operations {
			if (op.Operation == "asset_get" || op.Operation == "proposal_asset_get") && op.SaveAs != nil && *op.SaveAs == root {
				asset = true
			}
		}
		if len(projection.Fields) > 0 {
			rows, ok := value.([]any)
			if !ok {
				rows = []any{value}
			}
			limit := maxRecords
			if asset {
				limit = len(rows)
			}
			if projection.Limit != nil && *projection.Limit != 0 {
				limit = *projection.Limit
			}
			selected := []any{}
			for _, row := range rows[:sliceEnd(len(rows), limit)] {
				fields := map[string]any{}
				for _, field := range projection.Fields {
					v, err := ExtractField(row, field)
					if err != nil {
						return nil, err
					}
					fields[field] = v
				}
				selected = append(selected, fields)
			}
			value = selected
		} else if projection.Limit != nil {
			if rows, ok := value.([]any); ok {
				value = rows[:sliceEnd(len(rows), *projection.Limit)]
			}
		}
		name := strings.ReplaceAll(strings.TrimPrefix(projection.Ref, "$"), ".", "_")
		if projection.Name != nil && *projection.Name != "" {
			name = *projection.Name
		}
		if !asset {
			value = BoundValue(value, maxRecords)
		}
		result[name] = value
	}
	return result, nil
}

// FailedSaveNames suppresses projections only for failed names not present in
// saved. An earlier successful value survives a later failed operation reuse.
func FailedSaveNames(trace []map[string]any, saved map[string]any) map[string]bool {
	failed := map[string]bool{}
	for _, entry := range trace {
		if entry["status"] != "error" || entry["save_as"] == nil {
			continue
		}
		name := fmt.Sprint(entry["save_as"])
		if _, ok := saved[name]; !ok {
			failed[name] = true
		}
	}
	return failed
}
