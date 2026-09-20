package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/pyjson"
)

func TestProposalAccessReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-access.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Normalization []struct {
			Changes   []any
			Expected  []ProposalChange
			ErrorType string `json:"error_type"`
			Error     string
		}
		Authorization []struct {
			Policy  access.EffectivePolicy
			Changes []any
			Action  string
			Allowed bool
			Error   string
		}
		Visibility []struct {
			Policy       access.EffectivePolicy
			Changes      []any
			RequireWrite bool `json:"require_write"`
			Allowed      bool
			Error        string
			ErrorType    string `json:"error_type"`
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for i, c := range fixture.Normalization {
		got, err := NormalizeProposalChanges(c.Changes)
		if c.ErrorType != "" {
			if err == nil {
				t.Fatalf("normalization %d: accepted invalid changes %+v", i, c.Changes)
			}
			// Model/attribute diagnostics are different, but policy errors remain typed.
			var policy *Error
			if (c.ErrorType == "ServiceError") != errors.As(err, &policy) {
				t.Fatalf("normalization %d: wrong error %T %v", i, err, err)
			}
			if c.Error != "" {
				kind, _ := c.Changes[0].(map[string]any)
				_, str := kind["kind"].(string)
				if (str || kind["kind"] == nil) && err.Error() != c.Error {
					t.Fatalf("normalization %d: %v != %s", i, err, c.Error)
				}
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(jsonNormal(got), jsonNormal(c.Expected)) {
			t.Fatalf("normalization %d: %+v != %+v: %v", i, got, c.Expected, err)
		}
	}
	for i, c := range fixture.Authorization {
		changes, err := NormalizeProposalChanges(c.Changes)
		if err != nil {
			t.Fatal(err)
		}
		err = ValidateChangeAuthorization(c.Policy, changes, c.Action)
		if c.Allowed {
			if err != nil {
				t.Fatalf("auth %d: %v", i, err)
			}
		} else if err == nil || err.Error() != c.Error {
			t.Fatalf("auth %d: %v != %s", i, err, c.Error)
		}
	}
	for i, c := range fixture.Visibility {
		raw, err := pyjson.Dumps(map[string]any{"changes": c.Changes})
		if err != nil {
			t.Fatal(err)
		}
		record := control.ProposalRecord{ProposalID: "proposal", AuthorPrincipal: "author", PatchJSON: raw}
		got, err := CanAccessProposal(c.Policy, record, c.RequireWrite)
		requireErr := RequireProposalAccess(c.Policy, record, c.RequireWrite)
		if c.ErrorType != "" {
			var modelErr *ChangeValidationError
			if !errors.As(err, &modelErr) || requireErr == nil {
				t.Fatalf("visibility %d: %v %v", i, err, requireErr)
			}
			_ = modelErr.Error()
			continue
		}
		if err != nil || got != c.Allowed {
			t.Fatalf("visibility %d: %v %v != %v", i, got, err, c.Allowed)
		}
		if c.Error != "" {
			if requireErr == nil || requireErr.Error() != c.Error {
				t.Fatalf("require %d: %v != %s", i, requireErr, c.Error)
			}
		} else if requireErr != nil {
			t.Fatal(requireErr)
		}
	}
	t.Logf("%d normalization, %d authorization, %d visibility cases", len(fixture.Normalization), len(fixture.Authorization), len(fixture.Visibility))
}

func TestVisibleProposalPersistedErrors(t *testing.T) {
	ctx := context.Background()
	q, r := queueTest(t)
	policy := access.EffectivePolicy{Principal: r.AuthorPrincipal, Roles: []string{"proposer"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}
	got, err := q.VisibleProposal(ctx, policy, r.ProposalID)
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatal(got, err)
	}
	policy.Principal = "other"
	if _, err = q.VisibleProposal(ctx, policy, r.ProposalID); err == nil {
		t.Fatal("ownership bypass")
	}
	if _, err = q.VisibleProposal(ctx, policy, "missing"); err == nil {
		t.Fatal("missing record")
	}
	policy.Principal = r.AuthorPrincipal
	for _, raw := range []string{"{", `{}`, `{"changes":null}`} {
		r.PatchJSON = raw
		if _, err = CanAccessProposal(policy, r, false); err == nil {
			t.Fatal(raw)
		}
		if err = RequireProposalAccess(policy, r, false); err == nil {
			t.Fatal(raw)
		}
	}
}

func TestProposalAccessConcurrentPolicyIsolation(t *testing.T) {
	raw := `{"changes":[{"kind":"rename","path":"/public/a.md","new_path":"/private/a.md"}]}`
	record := control.ProposalRecord{ProposalID: "proposal", AuthorPrincipal: "author", PatchJSON: raw}
	policy := access.EffectivePolicy{Principal: "author", Roles: []string{"curator"}, ReadPrefixes: []string{"/public/"}, WritePrefixes: []string{"/public/"}, ProtectedReadPrefixes: []string{"/private/"}}
	admin := policy
	admin.Roles = []string{"curator", "admin"}
	admin.ReadPrefixes = []string{"/"}
	admin.WritePrefixes = []string{"/"}
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := policy
			want := false
			if i%2 == 0 {
				p = admin
				want = true
			}
			got, err := CanAccessProposal(p, record, true)
			if err != nil || got != want {
				t.Errorf("policy leak %v %v", got, err)
			}
		}()
	}
	wg.Wait()
	if record.PatchJSON != raw || len(policy.Roles) != 1 || policy.ReadPrefixes[0] != "/public/" {
		t.Fatal("input mutated")
	}
}

func FuzzProposalChanges(f *testing.F) {
	for _, raw := range []string{`[]`, `[{"kind":"patch","path":"/a.md"}]`, `[{"kind":"rename","path":"/a.md","new_path":"/b.md"}]`, `[{"kind":"create","path":"/a.md","concept_type":"x","title":"","body":"","tags":[]}]`, `[null]`, `[{"kind":"trash","path":"/trash/a.md"}]`} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 8192 {
			return
		}
		parsed, err := pyjson.Parse(raw)
		if err != nil {
			return
		}
		values, ok := parsed.([]any)
		if !ok {
			return
		}
		changes, err := NormalizeProposalChanges(values)
		if err != nil {
			return
		}
		policy := access.EffectivePolicy{Principal: "author", Roles: []string{"curator", "admin"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}
		first := ValidateChangeAuthorization(policy, changes, "review")
		encoded, err := json.Marshal(changes)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := pyjson.Parse(string(encoded))
		if err != nil {
			t.Fatal(err)
		}
		normalized, err := NormalizeProposalChanges(decoded.([]any))
		if err != nil || !reflect.DeepEqual(changes, normalized) {
			t.Fatal("unstable normalization", changes, normalized, err)
		}
		second := ValidateChangeAuthorization(policy, normalized, "review")
		if (first == nil) != (second == nil) || (first != nil && first.Error() != second.Error()) {
			t.Fatal("unstable policy", first, second)
		}
		patch, err := pyjson.Dumps(map[string]any{"changes": values})
		if err != nil {
			t.Fatal(err)
		}
		allowed, err := CanAccessProposal(policy, control.ProposalRecord{AuthorPrincipal: "author", PatchJSON: patch}, true)
		if err != nil || allowed != (first == nil) {
			t.Fatal("policy mismatch", allowed, first, err)
		}
	})
}
