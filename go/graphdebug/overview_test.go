package graphdebug

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/rcarmo/memento/go/access"
	"os"
	"testing"
)

func TestOverviewFixture(t *testing.T) {
	root, path := nodeDB(t)
	service := NewSnapshotService(root, path, fixtureControlDB(t))
	policy := access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}
	got, err := service.Overview(context.Background(), &policy, OverviewOptions{DirectNodeLimit: 2000, EdgeLimit: 12000})
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Overview Overview `json:"overview"`
	}
	raw, err := os.ReadFile("../testdata/parity/graph-snapshot-foundation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(fixture.Overview)
	if string(a) != string(b) {
		t.Fatal(string(a), string(b))
	}
}
func TestOverviewOrphanMetric(t *testing.T) {
	root, path := nodeDB(t)
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("DELETE FROM links")
	db.Close()
	service := NewSnapshotService(root, path, emptyControlDB(t))
	got, err := service.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 10, EdgeLimit: 10})
	if err != nil || got.Metrics.OrphanCount != 2 {
		t.Fatal(got, err)
	}
}
func TestOverviewTruncated(t *testing.T) {
	root, path := nodeDB(t)
	service := NewSnapshotService(root, path, emptyControlDB(t))
	got, err := service.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 1, EdgeLimit: 10, IncludeTrash: true})
	if err == nil || got.SchemaVersion != 0 {
		t.Fatal(got, err)
	}
}
