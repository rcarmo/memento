package service

import (
	"encoding/json"
	"errors"
	"strings"
)

type NeedleRouterConfig struct {
	Enabled        bool   `json:"enabled"`
	FFILibraryPath string `json:"ffi_library_path"`
	ModelPath      string `json:"model_path"`
	TokenizerPath  string `json:"tokenizer_path"`
}

func DefaultNeedleRouterConfig() NeedleRouterConfig {
	return NeedleRouterConfig{FFILibraryPath: "/usr/local/lib/memento/libmemento_needle_ffi.so", ModelPath: "/usr/local/share/memento/models/memento-router.ndl", TokenizerPath: "/usr/local/share/memento/models/needle.model"}
}
func (c NeedleRouterConfig) Validate() error {
	if strings.TrimSpace(c.FFILibraryPath) == "" || strings.TrimSpace(c.ModelPath) == "" || strings.TrimSpace(c.TokenizerPath) == "" {
		return errors.New("path values must not be empty")
	}
	return nil
}
func (c NeedleRouterConfig) Resolved(lookup func(string) (string, bool)) NeedleRouterConfig {
	if lookup == nil {
		return c
	}
	for _, item := range []struct {
		name   string
		target *string
	}{{"MEMENTO_NEEDLE_FFI_LIBRARY", &c.FFILibraryPath}, {"MEMENTO_NEEDLE_MODEL", &c.ModelPath}, {"MEMENTO_NEEDLE_TOKENIZER", &c.TokenizerPath}} {
		if value, ok := lookup(item.name); ok && strings.TrimSpace(value) != "" {
			*item.target = strings.TrimSpace(value)
		}
	}
	return c
}
func decodeNeedleConfig(raw json.RawMessage) (NeedleRouterConfig, error) {
	c := DefaultNeedleRouterConfig()
	if len(raw) == 0 {
		return c, nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return NeedleRouterConfig{}, err
	}
	if err := c.Validate(); err != nil {
		return NeedleRouterConfig{}, err
	}
	c.FFILibraryPath = strings.TrimSpace(c.FFILibraryPath)
	c.ModelPath = strings.TrimSpace(c.ModelPath)
	c.TokenizerPath = strings.TrimSpace(c.TokenizerPath)
	return c, nil
}
