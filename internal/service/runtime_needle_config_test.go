package service

import (
	"encoding/json"
	"testing"
)

func TestNeedleRouterConfig(t *testing.T) {
	defaults := DefaultNeedleRouterConfig()
	if defaults.Enabled || defaults.WorkerMode != "subprocess" || defaults.WorkerPath != "/usr/local/bin/memento-needle-go" || defaults.FP32ModelPath != "/usr/local/share/memento/models/memento-router.nfp32" || defaults.FFILibraryPath != "/usr/local/lib/memento/libmemento_needle_ffi.so" || defaults.ModelPath != "/usr/local/share/memento/models/memento-router.ndl" || defaults.TokenizerPath != "/usr/local/share/memento/models/needle.model" {
		t.Fatal(defaults)
	}
	if err := defaults.Validate(); err != nil {
		t.Fatal(err)
	}
	for index, mutate := range []func(*NeedleRouterConfig){
		func(c *NeedleRouterConfig) { c.WorkerMode = "bad" }, func(c *NeedleRouterConfig) { c.WorkerPath = "" },
		func(c *NeedleRouterConfig) { c.FP32ModelPath = "" }, func(c *NeedleRouterConfig) { c.WorkerTimeoutSeconds = 0 },
		func(c *NeedleRouterConfig) { c.FFILibraryPath = " " }, func(c *NeedleRouterConfig) { c.ModelPath = "" }, func(c *NeedleRouterConfig) { c.TokenizerPath = "" },
	} {
		c := defaults
		mutate(&c)
		if c.Validate() == nil {
			t.Fatal(index)
		}
	}
	resolved := defaults.Resolved(func(name string) (string, bool) {
		values := map[string]string{"MEMENTO_NEEDLE_WORKER": " /worker ", "MEMENTO_NEEDLE_FP32_MODEL": " /fp32 ", "MEMENTO_NEEDLE_FFI_LIBRARY": " /ffi ", "MEMENTO_NEEDLE_MODEL": "/model", "MEMENTO_NEEDLE_TOKENIZER": "/tokenizer"}
		value, ok := values[name]
		return value, ok
	})
	if resolved.WorkerPath != "/worker" || resolved.FP32ModelPath != "/fp32" || resolved.FFILibraryPath != "/ffi" || resolved.ModelPath != "/model" || resolved.TokenizerPath != "/tokenizer" {
		t.Fatal(resolved)
	}
	if got := defaults.Resolved(nil); got != defaults {
		t.Fatal(got)
	}
}
func TestDecodeNeedleConfig(t *testing.T) {
	c, err := DecodeNeedleRouterConfig(nil)
	if err != nil || c != DefaultNeedleRouterConfig() {
		t.Fatal(c, err)
	}
	raw := json.RawMessage(`{"enabled":false,"worker_mode":"in_process","worker_path":" /worker ","worker_timeout_seconds":12,"fp32_model_path":" /fp32 ","model_path":" /model ","tokenizer_path":"/tokenizer","ffi_library_path":"/ffi"}`)
	c, err = DecodeNeedleRouterConfig(raw)
	if err != nil || c.WorkerMode != "in_process" || c.WorkerPath != "/worker" || c.FP32ModelPath != "/fp32" || c.ModelPath != "/model" {
		t.Fatal(c, err)
	}
	for _, raw := range []json.RawMessage{[]byte(`{"extra":1}`), []byte(`{"model_path":""}`), []byte(`bad`)} {
		if _, err = DecodeNeedleRouterConfig(raw); err == nil {
			t.Fatal(string(raw))
		}
	}
}
