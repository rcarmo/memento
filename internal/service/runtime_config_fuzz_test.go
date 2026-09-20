package service

import (
	"encoding/json"
	"testing"
)

func FuzzRuntimeConfig(f *testing.F) {
	for _, raw := range []string{
		`{}`,
		`{"schema_version":2,"repository":{"root_path":"/tmp/memento","bundle_root":"/"}}`,
		`{"schema_version":1,"repository":{"root_path":"x","bundle_root":"/"}}`,
		`{"schema_version":2,"repository":{"root_path":"","bundle_root":"/"}} trailing`,
		`null`,
	} {
		f.Add([]byte(raw))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		config, err := parseRuntimeConfig(raw)
		if err != nil {
			return
		}
		if config.SchemaVersion != 2 || config.Repository.RootPath == "" || config.Repository.BundleRoot != "/" {
			t.Fatalf("accepted invalid config: %#v", config)
		}
		encoded, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = parseRuntimeConfig(encoded); err != nil {
			t.Fatalf("accepted config did not round-trip: %v", err)
		}
	})
}
