package service

import (
	"context"
	"strings"
	"testing"
)

func TestAccessToolArgumentValidation(t *testing.T) {
	store := accessReaderStub{}
	if _, err := callAccessTool(context.Background(), store, "admin", "unknown", nil); err == nil {
		t.Fatal("unsupported tool")
	}
	invalid := []struct {
		field string
		value any
		want  string
	}{
		{"name", 1, "name must be a string"},
		{"roles", "reader", "roles must be an array of strings"},
		{"roles", []any{"reader", 1}, "roles must be an array of strings"},
		{"read_prefixes", "bad", "read_prefixes must be an array of strings"},
		{"write_prefixes", "bad", "write_prefixes must be an array of strings"},
		{"idempotency_key", 1, "idempotency_key must be a string"},
	}
	base := map[string]any{"name": "agent", "roles": []any{"reader"}, "read_prefixes": []any{"/"}, "write_prefixes": []any{}, "idempotency_key": "key"}
	for _, tc := range invalid {
		args := map[string]any{}
		for key, value := range base {
			args[key] = value
		}
		args[tc.field] = tc.value
		_, _, _, _, _, err := requiredCreate(args)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatal(tc, err)
		}
	}
	calls := []struct {
		name string
		args map[string]any
	}{
		{"access_principal_create", base},
		{"access_principal_update", map[string]any{"name": "agent", "roles": []any{"reader"}, "read_prefixes": []any{"/"}, "write_prefixes": []any{}}},
		{"access_principal_rename", map[string]any{"name": "agent", "new_name": "worker"}},
		{"access_principal_disable", map[string]any{"name": "agent"}},
		{"access_principal_enable", map[string]any{"name": "agent"}},
		{"access_credential_rotate", map[string]any{"name": "agent", "idempotency_key": "key"}},
		{"access_principal_revoke", map[string]any{"name": "agent"}},
		{"access_principal_delete", map[string]any{"name": "agent"}},
	}
	for _, tc := range calls {
		if _, err := callAccessTool(context.Background(), store, "admin", tc.name, tc.args); err == nil {
			t.Fatal(tc.name)
		}
	}
	defer func() {
		if recover() == nil {
			t.Fatal("array invariant did not panic")
		}
	}()
	mustStrings("bad")
}
