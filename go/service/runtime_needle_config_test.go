package service

import (
	"encoding/json"
	"testing"
)

func TestNeedleRouterConfig(t *testing.T) {
	defaults := DefaultNeedleRouterConfig()
	if defaults.Enabled || defaults.FFILibraryPath != "/usr/local/lib/memento/libmemento_needle_ffi.so" || defaults.ModelPath != "/usr/local/share/memento/models/memento-router.ndl" || defaults.TokenizerPath != "/usr/local/share/memento/models/needle.model" {
		t.Fatal(defaults)
	}
	if err := defaults.Validate(); err != nil {
		t.Fatal(err)
	}
	for index, mutate := range []func(*NeedleRouterConfig){func(c *NeedleRouterConfig) { c.FFILibraryPath = " " }, func(c *NeedleRouterConfig) { c.ModelPath = "" }, func(c *NeedleRouterConfig) { c.TokenizerPath = "" }} {
		c := defaults
		mutate(&c)
		if c.Validate() == nil {
			t.Fatal(index)
		}
	}
	resolved := defaults.Resolved(func(name string) (string, bool) {
		values := map[string]string{"MEMENTO_NEEDLE_FFI_LIBRARY": " /ffi ", "MEMENTO_NEEDLE_MODEL": "/model", "MEMENTO_NEEDLE_TOKENIZER": "/tokenizer"}
		value, ok := values[name]
		return value, ok
	})
	if resolved.FFILibraryPath != "/ffi" || resolved.ModelPath != "/model" || resolved.TokenizerPath != "/tokenizer" {
		t.Fatal(resolved)
	}
	if got := defaults.Resolved(nil); got != defaults {
		t.Fatal(got)
	}
}
func TestDecodeNeedleConfig(t *testing.T) {
	c, err := decodeNeedleConfig(nil)
	if err != nil || c != DefaultNeedleRouterConfig() {
		t.Fatal(c, err)
	}
	raw := json.RawMessage(`{"enabled":false,"model_path":" /model ","tokenizer_path":"/tokenizer","ffi_library_path":"/ffi"}`)
	c, err = decodeNeedleConfig(raw)
	if err != nil || c.ModelPath != "/model" {
		t.Fatal(c, err)
	}
	for _, raw := range []json.RawMessage{[]byte(`{"extra":1}`), []byte(`{"model_path":""}`), []byte(`bad`)} {
		if _, err = decodeNeedleConfig(raw); err == nil {
			t.Fatal(string(raw))
		}
	}
}
