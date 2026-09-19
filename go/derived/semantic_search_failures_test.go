package derived

import (
	"errors"
	"testing"
)

type semanticRowsStub struct {
	next              bool
	values            []any
	scanErr, finalErr error
}

func (r *semanticRowsStub) Next() bool {
	if r.next {
		r.next = false
		return true
	}
	return false
}
func (r *semanticRowsStub) Scan(targets ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for i, target := range targets {
		switch value := target.(type) {
		case *string:
			*value = r.values[i].(string)
		case *[]byte:
			*value = r.values[i].([]byte)
		}
	}
	return nil
}
func (r *semanticRowsStub) Err() error   { return r.finalErr }
func (r *semanticRowsStub) Close() error { return nil }
func TestReadSemanticRowsFailures(t *testing.T) {
	boom := errors.New("boom")
	if _, err := readSemanticRows(&semanticRowsStub{next: true, scanErr: boom}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	values := []any{"id", "/a", "A", "concept", "active", "bad", "A", semanticBlob(1, 0)}
	if _, err := readSemanticRows(&semanticRowsStub{next: true, values: values}); err == nil {
		t.Fatal("tags")
	}
	if _, err := readSemanticRows(&semanticRowsStub{finalErr: boom}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	values[5] = "null"
	rows, err := readSemanticRows(&semanticRowsStub{next: true, values: values})
	if err != nil || rows[0].result.Tags == nil {
		t.Fatal(rows, err)
	}
}
