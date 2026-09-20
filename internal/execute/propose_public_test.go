package execute

import (
	"errors"
	"testing"
)

func TestValidateProposalChanges(t *testing.T) {
	rows, err := ValidateProposalChanges([]any{map[string]any{"kind": "create", "path": "/a", "concept_type": "concept", "title": "A", "body": "B"}})
	if err != nil || len(rows) != 1 || rows[0]["kind"] != "create" {
		t.Fatal(rows, err)
	}
	boom := errors.New("boom")
	if _, err = validateProposalChanges(nil, func() (*Arguments, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if _, err = ValidateProposalChanges([]any{map[string]any{"kind": "bad"}}); err == nil {
		t.Fatal("bad")
	}
}
