package service

import (
	"github.com/rcarmo/memento/go/access"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimeConfigAndPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	raw := `{"repository":{"root_path":"/tmp/memento/../runtime"},"authorization":{"principals":{},"protected_read_prefixes":[]}}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadRuntimeConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	got := RuntimePathsFor(config)
	if got.Root != "/tmp/runtime" || got.Repository.BareDir != "/tmp/runtime/repo.git" || got.ControlDB != "/tmp/runtime/control.sqlite" || got.DerivedDB != "/tmp/runtime/derived.sqlite" || got.WriterLock != "/tmp/runtime/locks/writer.lock" {
		t.Fatal(got)
	}
	for _, raw := range []string{"bad", `{}`, `{"repository":{"root_path":""},"authorization":{}}`, strings.TrimSuffix(raw, "}") + `} trailing`, `{"repository":{"root_path":"x","extra":1},"authorization":{}}`} {
		_ = os.WriteFile(path, []byte(raw), 0600)
		if _, err = LoadRuntimeConfig(path); err == nil {
			t.Fatal(raw)
		}
	}
	if _, err = LoadRuntimeConfig(path + "-missing"); err == nil {
		t.Fatal("missing")
	}
}
func TestStaticBearerPrincipals(t *testing.T) {
	config := access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"z": {TokenEnv: "Z", Roles: []string{"reader"}}, "a": {TokenEnv: "A", Roles: []string{"admin"}}}}
	env := map[string]string{"A": " one ", "Z": "two"}
	tokens, err := StaticBearerPrincipals(config, func(name string) (string, bool) { value, ok := env[name]; return value, ok })
	if err != nil || len(tokens) != 2 || tokens[0].Token != "one" || tokens[0].Principal.Name != "a" || tokens[0].Principal.Metadata["token_env"] != "A" {
		t.Fatal(tokens, err)
	}
	config.Principals["a"] = access.NamespacePolicy{}
	if !reflect.DeepEqual(tokens[0].Principal.Roles, []string{"admin"}) {
		t.Fatal("alias")
	}
	config.Principals["a"] = access.NamespacePolicy{TokenEnv: "A", Roles: []string{"admin"}}
	for _, mutate := range []func(){func() { delete(env, "A") }, func() { env["A"] = " " }, func() { env["A"] = "two" }} {
		env = map[string]string{"A": "one", "Z": "two"}
		mutate()
		if _, err = StaticBearerPrincipals(config, func(name string) (string, bool) { value, ok := env[name]; return value, ok }); err == nil {
			t.Fatal(env)
		}
	}
	if _, err = StaticBearerPrincipals(config, nil); err == nil {
		t.Fatal("real missing env")
	}
}
