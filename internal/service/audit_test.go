package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

type auditProvider func(context.Context, access.EffectivePolicy) (AuditOverview, error)

func (f auditProvider) Overview(ctx context.Context, p access.EffectivePolicy) (AuditOverview, error) {
	return f(ctx, p)
}
func TestAuditDispatchReference(t *testing.T) {
	testToolReference(t, "audit-tool-dispatch.json", auditToolDefinitions)
}
func TestAuditReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/service-audit.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Files       map[string]string
		Nodes       []AuditGraphNode
		Diagnostics []AuditDiagnostic
		Cases       []struct {
			Mode      string
			Arguments map[string]any
			Policy    access.EffectivePolicy
			Expected  map[string]any
			Calls     []access.EffectivePolicy
		}
		Policies []struct{ Policy, Expected access.EffectivePolicy }
		Cursor   string
		Cursors  []struct {
			Cursor   *string
			Expected *[3]string
			Error    string
		}
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	c, _ := rebaseTest(t)
	c.Queue.Paths.CurrentDir = t.TempDir()
	files := map[string][]byte{}
	for p, v := range f.Files {
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			t.Fatal(err)
		}
		files[p] = b
	}
	installAssetFiles(t, c.Queue.Paths.CurrentDir, files)
	db, err := control.Connect(context.Background(), fmt.Sprintf("%s/failure.sqlite", t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, sqlError := db.Exec("SELECT * FROM nonexistent")
	for i, tc := range f.Cases {
		t.Run(fmt.Sprintf("%d-%s", i, tc.Mode), func(t *testing.T) {
			calls := []access.EffectivePolicy{}
			c.AuditGraph = auditProvider(func(_ context.Context, policy access.EffectivePolicy) (AuditOverview, error) {
				calls = append(calls, policy)
				o := AuditOverview{IndexRevision: "main", Nodes: f.Nodes, Diagnostics: f.Diagnostics}
				switch tc.Mode {
				case "stale":
					o.Stale = true
				case "mismatch":
					o.IndexRevision = "old"
				case "snapshot-stale":
					return o, &GraphSnapshotError{"STALE"}
				case "snapshot-unavailable":
					return o, &GraphSnapshotError{"missing"}
				case "oserror":
					return o, &fs.PathError{Op: "read", Path: "synthetic", Err: os.ErrPermission}
				case "sqlite":
					return o, sqlError
				}
				return o, nil
			})
			if tc.Mode == "none" {
				c.AuditGraph = nil
			}
			o := AuditOptions{Limit: 50}
			for key, target := range map[string]**string{"path": &o.Path, "rule": &o.Rule, "severity": &o.Severity, "cursor": &o.Cursor} {
				if value, ok := tc.Arguments[key].(string); ok {
					*target = &value
				}
			}
			if n, ok := tc.Arguments["limit"].(float64); ok {
				o.Limit = int(n)
			}
			data, options, err := c.audit(context.Background(), ProposalActor{Policy: tc.Policy}, o, fakeRepo(nil), repository.AuditRepository)
			var result any
			if err != nil {
				result, err = FailureEnvelope(err)
			} else {
				result, err = c.Queue.successEnvelope(data, options, fakeRepo(nil))
			}
			if err != nil || !reflect.DeepEqual(jsonNormal(result), tc.Expected) || !reflect.DeepEqual(calls, tc.Calls) {
				t.Fatal(tc.Arguments, result, tc.Expected, err, calls, tc.Calls)
			}
		})
	}
	for _, tc := range f.Policies {
		if got := writableAuditPolicy(tc.Policy); !reflect.DeepEqual(got, tc.Expected) {
			t.Fatal(got, tc.Expected)
		}
	}
	filters := map[string]any{"path": nil, "rule": nil, "severity": nil}
	encoded, err := encodeAuditCursor([3]string{"info", "pending_proposals", "a"}, "main", filters)
	if err != nil || encoded != f.Cursor {
		t.Fatal(encoded, f.Cursor, err)
	}
	for _, tc := range f.Cursors {
		got, err := decodeAuditCursor(tc.Cursor, "main", filters)
		if tc.Error != "" {
			if err == nil || err.Error() != tc.Error {
				t.Fatal(tc.Cursor, got, err)
			}
		} else if err != nil || !reflect.DeepEqual(got, tc.Expected) {
			t.Fatal(tc.Cursor, got, tc.Expected, err)
		}
	}
}
