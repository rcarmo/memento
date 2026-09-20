package service

import (
	"encoding/json"
	"errors"
	"strings"
)

type NeedleRouterConfig struct {
	Enabled              bool    `json:"enabled"`
	WorkerMode           string  `json:"worker_mode"`
	WorkerPath           string  `json:"worker_path"`
	WorkerTimeoutSeconds float64 `json:"worker_timeout_seconds"`
	FP32ModelPath        string  `json:"fp32_model_path"`
	FFILibraryPath       string  `json:"ffi_library_path"`
	ModelPath            string  `json:"model_path"`
	TokenizerPath        string  `json:"tokenizer_path"`
}

func DefaultNeedleRouterConfig() NeedleRouterConfig {
	return NeedleRouterConfig{
		WorkerMode: "subprocess", WorkerPath: "/usr/local/bin/memento-needle-go", WorkerTimeoutSeconds: 10,
		FP32ModelPath:  "/usr/local/share/memento/models/memento-router.nfp32",
		FFILibraryPath: "/usr/local/lib/memento/libmemento_needle_ffi.so",
		ModelPath:      "/usr/local/share/memento/models/memento-router.ndl",
		TokenizerPath:  "/usr/local/share/memento/models/needle.model",
	}
}
func (c NeedleRouterConfig) Validate() error {
	if c.WorkerMode != "subprocess" && c.WorkerMode != "in_process" {
		return errors.New("needle worker_mode must be subprocess or in_process")
	}
	if strings.TrimSpace(c.FFILibraryPath) == "" || strings.TrimSpace(c.ModelPath) == "" || strings.TrimSpace(c.TokenizerPath) == "" || strings.TrimSpace(c.FP32ModelPath) == "" || strings.TrimSpace(c.WorkerPath) == "" {
		return errors.New("path values must not be empty")
	}
	if c.WorkerTimeoutSeconds <= 0 || c.WorkerTimeoutSeconds > 120 {
		return errors.New("needle worker_timeout_seconds must be greater than zero and at most 120")
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
	}{{"MEMENTO_NEEDLE_WORKER", &c.WorkerPath}, {"MEMENTO_NEEDLE_FP32_MODEL", &c.FP32ModelPath}, {"MEMENTO_NEEDLE_FFI_LIBRARY", &c.FFILibraryPath}, {"MEMENTO_NEEDLE_MODEL", &c.ModelPath}, {"MEMENTO_NEEDLE_TOKENIZER", &c.TokenizerPath}} {
		if value, ok := lookup(item.name); ok && strings.TrimSpace(value) != "" {
			*item.target = strings.TrimSpace(value)
		}
	}
	return c
}
func DecodeNeedleRouterConfig(raw json.RawMessage) (NeedleRouterConfig, error) {
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
	c.WorkerPath = strings.TrimSpace(c.WorkerPath)
	c.FP32ModelPath = strings.TrimSpace(c.FP32ModelPath)
	c.FFILibraryPath = strings.TrimSpace(c.FFILibraryPath)
	c.ModelPath = strings.TrimSpace(c.ModelPath)
	c.TokenizerPath = strings.TrimSpace(c.TokenizerPath)
	return c, nil
}
