// Package processnice applies a bounded lower CPU scheduling priority to model workers.
package processnice

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	EnvVar          = "MEMENTO_EMBED_GO_NICE"
	DefaultPriority = 15
)

var (
	getPriority = getPriorityImpl
	setPriority = setPriorityImpl
)

func Validate(priority int) error {
	if priority < 0 || priority > 19 {
		return fmt.Errorf("nice priority %d out of range [0,19]", priority)
	}
	return nil
}

func Parse(raw string) (int, error) {
	text := strings.TrimSpace(raw)
	priority, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q", EnvVar, raw)
	}
	if err := Validate(priority); err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", EnvVar, raw, err)
	}
	return priority, nil
}

func Resolve(lookup func(string) (string, bool)) (int, error) {
	if lookup == nil {
		return DefaultPriority, nil
	}
	raw, ok := lookup(EnvVar)
	if !ok || strings.TrimSpace(raw) == "" {
		return DefaultPriority, nil
	}
	return Parse(raw)
}

func Apply(priority int) error {
	if err := Validate(priority); err != nil {
		return err
	}
	if priority == 0 {
		return nil
	}
	current, err := getPriority()
	if err != nil {
		return fmt.Errorf("get current nice priority: %w", err)
	}
	if current >= priority {
		return nil
	}
	if err := setPriority(priority); err != nil {
		return fmt.Errorf("set nice priority %d: %w", priority, err)
	}
	return nil
}
