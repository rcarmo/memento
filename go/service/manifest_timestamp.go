package service

import (
	"errors"
	"time"

	"github.com/rcarmo/memento/go/internal/pydatetime"
	"github.com/rcarmo/memento/go/umcp"
)

func manifestTimestamp(value any) (time.Time, error) {
	stamp, err := pydatetime.ParseAware(value)
	switch {
	case err == nil:
		return stamp, nil
	case errors.Is(err, pydatetime.ErrNaive):
		return time.Time{}, manifestInvalid("manifest timestamps must be timezone-aware")
	case errors.Is(err, pydatetime.ErrOverflow):
		return time.Time{}, umcp.ExecutionError{Type: "OverflowError", Message: "date value out of range"}
	default:
		return time.Time{}, manifestInvalid("manifest timestamps must be ISO 8601 values")
	}
}
