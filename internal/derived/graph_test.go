package derived

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
)

func graphReferenceStore(t *testing.T) (ContentStore, []byte) {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/parity/derived-graph.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Files map[string]string }
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for path, text := range fixture.Files {
		file := filepath.Join(root, path)
		if err = os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(file, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := testStore(t)
	if err = s.Rebuild(context.Background(), root, "r1"); err != nil {
		t.Fatal(err)
	}
	return s, raw
}
func TestGraphReference(t *testing.T) {
	s, raw := graphReferenceStore(t)
	ctx := context.Background()
	var fixture struct {
		Graphs []struct {
			Policy    access.EffectivePolicy
			Center    string
			Depth     int
			Expected  GraphNeighborhood
			ErrorType string `json:"error_type"`
		}
		Statuses []struct {
			Policy   access.EffectivePolicy
			Expected StatusSnapshot
		}
		Metrics []struct {
			ID        string
			Expected  GraphMetrics
			ErrorType string `json:"error_type"`
		}
		Waits []struct {
			State, Expected IndexState
			Timeout         float64
			Error           string
			Reads           int
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for i, c := range fixture.Graphs {
		got, err := s.Graph(ctx, c.Policy, c.Center, GraphOptions{Depth: c.Depth})
		if c.ErrorType != "" {
			var missing *ConceptNotFoundError
			if !errors.As(err, &missing) || missing.Error() != c.Center {
				t.Fatal(i, got, err)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(got, c.Expected) {
			t.Fatal(i, got, c.Expected, err)
		}
	}
	for _, c := range fixture.Statuses {
		got, err := s.Status(ctx, c.Policy)
		if err != nil || !reflect.DeepEqual(got, c.Expected) {
			t.Fatal(got, c.Expected, err)
		}
	}
	for _, c := range fixture.Metrics {
		got, err := s.Metrics(ctx, c.ID)
		if c.ErrorType != "" {
			if err == nil {
				t.Fatal(c.ID)
			}
		} else if err != nil || got != c.Expected {
			t.Fatal(got, c.Expected, err)
		}
	}
	for _, c := range fixture.Waits {
		if err := s.SetRepoRevision(ctx, c.State.RepoRevision); err != nil {
			t.Fatal(err)
		}
		if err := setState(ctx, s.DB, "index_revision", c.State.IndexRevision); err != nil {
			t.Fatal(err)
		}
		if err := setState(ctx, s.DB, "status", c.State.Status); err != nil {
			t.Fatal(err)
		}
		now := time.Unix(0, 0)
		sleeps := 0
		got, err := s.waitForFreshness(ctx, time.Duration(c.Timeout*float64(time.Second)), func() time.Time { return now }, func(_ context.Context, delay time.Duration) error {
			sleeps++
			now = now.Add(delay)
			return setState(ctx, s.DB, "index_revision", c.State.RepoRevision)
		})
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(got, err, c.Error)
			}
		} else if err != nil || got != c.Expected {
			t.Fatal(got, err, c.Expected)
		}
		if sleeps+1 != c.Reads {
			t.Fatal(sleeps, c.Reads)
		}
	}
}
func TestGraphAndStateSQLFailures(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	installConcept(t, root)
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	for _, kind := range []string{"graph", "status", "state", "metrics"} {
		t.Run(kind, func(t *testing.T) {
			run := func(n int) (int, error) {
				s, f := faultStore(t)
				if err := s.Rebuild(ctx, root, "r"); err != nil {
					t.Fatal(err)
				}
				f.remaining = n
				f.count = 0
				var err error
				switch kind {
				case "graph":
					_, err = s.Graph(ctx, policy, "id", GraphOptions{Depth: 2})
				case "status":
					_, err = s.Status(ctx, policy)
				case "state":
					_, err = s.State(ctx)
				case "metrics":
					_, err = s.Metrics(ctx, "id")
				}
				s.DB.Close()
				return f.count, err
			}
			count, err := run(0)
			if err != nil {
				t.Fatal(err)
			}
			for n := 1; n <= count; n++ {
				if _, err = run(n); err == nil {
					t.Fatal("ignored failure", kind, n)
				}
			}
		})
	}
	// A multi-node graph executes the per-neighbor query/metric failure paths.
	original, raw := graphReferenceStore(t)
	original.DB.Close()
	var fixture struct{ Files map[string]string }
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for path, text := range fixture.Files {
		file := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(n int) (int, error) {
		s, f := faultStore(t)
		if err := s.Rebuild(ctx, root, "r"); err != nil {
			t.Fatal(err)
		}
		f.remaining = n
		f.count = 0
		_, err := s.Graph(ctx, policy, "a", GraphOptions{Depth: 2})
		s.DB.Close()
		return f.count, err
	}
	count, err := run(0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= count; i++ {
		if _, err = run(i); err == nil {
			t.Fatal("ignored neighbor error", i)
		}
	}
}
func TestGraphRowsAndStrictFreshness(t *testing.T) {
	for _, scan := range []bool{false, true} {
		for _, broken := range []bool{false, true} {
			r := &badRows{}
			if scan {
				r.scan = io.ErrClosedPipe
			} else {
				r.iteration = io.ErrClosedPipe
			}
			var err error
			if broken {
				_, err = brokenRows(r, access.EffectivePolicy{})
			} else {
				_, err = neighborRows(r)
			}
			if !errors.Is(err, io.ErrClosedPipe) || !r.closed {
				t.Fatal(err)
			}
		}
	}
	ctx := context.Background()
	s, _ := graphReferenceStore(t)
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/public/"}}
	if _, err := s.Graph(ctx, policy, "a", GraphOptions{Depth: 1, Strict: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SearchLexical(ctx, policy, SearchOptions{Query: "Title", Syntax: "plain", Strict: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRepoRevision(ctx, "r2"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Graph(ctx, policy, "a", GraphOptions{Depth: 1, Strict: true}); err == nil {
		t.Fatal("stale graph")
	}
	if _, err := s.SearchLexical(ctx, policy, SearchOptions{Query: "Title", Syntax: "plain", Strict: true}); err == nil {
		t.Fatal("stale search")
	}
	sleeps := 0
	if _, err := s.waitForFreshness(ctx, time.Second, time.Now, func(context.Context, time.Duration) error { sleeps++; return io.ErrClosedPipe }); !errors.Is(err, io.ErrClosedPipe) || sleeps != 1 {
		t.Fatal(err, sleeps)
	}
	if err := sleepFreshness(ctx, time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := sleepFreshness(canceled, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.WaitForFreshness(canceled, time.Second); err == nil {
		t.Fatal("cancelled state")
	}
	if err := setState(ctx, s.DB, "status", "quarantined"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Graph(ctx, policy, "a", GraphOptions{}); err == nil {
		t.Fatal("quarantined graph")
	}
	if _, err := s.DB.Exec("DELETE FROM index_state WHERE key='index_revision'"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.State(ctx); err == nil {
		t.Fatal("missing state")
	}
}

func TestGraphMalformedNeighborsAndTieOrder(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	root := t.TempDir()
	installConcept(t, root)
	if err := s.Rebuild(ctx, root, "r"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`INSERT INTO links VALUES('id','bad','/bad.md','/bad.md',NULL,'markdown','resolved','r','r');INSERT INTO concepts SELECT 'bad','/bad.md',type,title,description,status,tags_json,aliases_json,body,content_hash,updated_at,repo_revision FROM concepts WHERE id='id';`); err != nil {
		t.Fatal(err)
	}
	// A malformed legacy row in a view must surface as an error, not be hidden by
	// policy or returned as a partial graph.
	if _, err := s.DB.Exec(`ALTER TABLE concepts RENAME TO old_concepts;CREATE VIEW concepts AS SELECT id,path,CASE WHEN id='bad' THEN NULL ELSE title END AS title FROM old_concepts`); err != nil {
		t.Fatal(err)
	}
	if _, err := collectNeighbors(ctx, s.DB, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, "id", "outbound", 1); err == nil {
		t.Fatal("malformed neighbor")
	}
}

func TestGraphDuplicatePathLegacyOrdering(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`DROP TABLE concepts;CREATE TABLE concepts(id TEXT,path TEXT,title TEXT);INSERT INTO concepts VALUES('center','/center.md','Center'),('z','/same.md','Z'),('a','/same.md','A');INSERT INTO links VALUES('center','z','/same.md','/same.md',NULL,'markdown','resolved','r','r'),('center','a','/same.md','/same.md',NULL,'markdown','resolved','r','r')`); err != nil {
		t.Fatal(err)
	}
	edges, err := collectNeighbors(ctx, s.DB, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, "center", "outbound", 1)
	if err != nil || len(edges) != 2 || edges[0].ConceptID != "a" || edges[1].ConceptID != "z" {
		t.Fatal(edges, err)
	}
}

func TestFreshnessConcurrentAdvance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s, _ := graphReferenceStore(t)
	if err := s.SetRepoRevision(ctx, "r2"); err != nil {
		t.Fatal(err)
	}
	sleeping := make(chan struct{})
	release := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := s.waitForFreshness(ctx, time.Second, time.Now, func(ctx context.Context, delay time.Duration) error {
			close(sleeping)
			<-release
			return sleepFreshness(ctx, delay)
		})
		result <- err
	}()
	<-sleeping
	if err := setState(ctx, s.DB, "index_revision", "r2"); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if err := setState(ctx, s.DB, "quarantine_path", "index.quarantine.sqlite"); err != nil {
		t.Fatal(err)
	}
	state, err := s.State(ctx)
	if err != nil || state.QuarantinePath == nil || *state.QuarantinePath != "index.quarantine.sqlite" {
		t.Fatal(state, err)
	}
}
