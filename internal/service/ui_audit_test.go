package service

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/umcp"
)

// TestUIAuditServer is an opt-in disposable browser fixture, never a production
// endpoint. tools/browser/audit.mjs owns its startup and shutdown.
func TestUIAuditServer(t *testing.T) {
	ready := os.Getenv("MEMENTO_UI_AUDIT_READY")
	if ready == "" {
		t.Skip("browser fixture only")
	}
	t.Setenv("MEMENTO_ADMIN_MASTER_KEY", "isolated-browser-master")
	t.Setenv("UI_ADMIN_TOKEN", "ui-admin-token")
	t.Setenv("UI_READER_TOKEN", "ui-reader-token")
	root := t.TempDir()
	seed := t.TempDir()
	files := map[string]string{}
	for i := 0; i < 120; i++ {
		namespace, kind := "projects", "project"
		if i%3 == 1 {
			namespace, kind = "skills", "concept"
		}
		if i%3 == 2 {
			namespace, kind = "private", "service"
		}
		files[fmt.Sprintf("/%s/node-%03d.md", namespace, i)] = fmt.Sprintf("---\nid: '%08x-0000-4000-8000-000000000000'\ntype: %s\ntitle: Node %03d\nstatus: active\ntags: [audit, group%d]\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: ui-audit\n---\nAudit body %d [next](/projects/node-000.md)\n", i+1, kind, i, i%6, i)
	}
	files["/trash/old.md"] = "---\nid: 'eeeeeeee-0000-4000-8000-000000000000'\ntype: concept\ntitle: Trashed\nstatus: tombstone\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: ui-audit\n---\nOld\n"
	installMutationFiles(t, seed, files)
	var config RuntimeConfig
	config.Repository.RootPath = root
	config.Limits.MaxConceptBytes = 65536
	config.Authorization = access.AuthorizationConfig{ProtectedReadPrefixes: []string{"/private/"}, Principals: map[string]access.NamespacePolicy{
		"admin":           {TokenEnv: "UI_ADMIN_TOKEN", Roles: []string{"admin", "reader", "proposer", "curator"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}},
		"projects-reader": {TokenEnv: "UI_READER_TOKEN", Roles: []string{"reader"}, ReadPrefixes: []string{"/projects/"}},
	}}
	graph := DefaultGraphExplorerConfig()
	graph.Enabled = true
	ctx := context.Background()
	runtime, server, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "compact", Limits: endpointLimits(), BootstrapSeed: seed, Graph: graph.HTTPConfig()})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)
	server.SetNotificationOutput(nil)
	db, err := derived.Connect(ctx, runtime.Paths.DerivedDB)
	if err != nil {
		t.Fatal(err)
	}
	var revision string
	if err = db.QueryRow("SELECT value FROM index_state WHERE key='repo_revision'").Scan(&revision); err != nil {
		t.Fatal(err)
	}
	vector := make([]byte, 8)
	binary.LittleEndian.PutUint32(vector, math.Float32bits(1))
	if _, err = db.Exec(`INSERT INTO concept_embeddings(concept_id,path,embedding_text_hash,model_id,dimensions,embedding_revision,status,model_revision,embedding_blob,embedding_norm,updated_at) SELECT id,path,'fixture','test',2,?,'ready','test',?,1,'now' FROM concepts; UPDATE index_state SET value=? WHERE key='semantic_embedding_revision'`, revision, vector, revision); err != nil {
		t.Fatal(err)
	}
	db.Close()
	// Use real snapshot/HTTP dispatch for all graph reads; the no-worker response
	// is deliberate. Refresh-success/failure cases use controlled browser routes.
	transport, err := umcp.NewStreamableHTTP(server, umcp.DefaultHTTPOptions(), runtime.HTTPHooks)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	host := httptest.NewServer(transport)
	defer host.Close()
	if err = os.WriteFile(ready, []byte(host.URL), 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(ready)
	deadline := time.Now().Add(12 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Clean(ready) + ".stop"); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture exceeded deadline")
}
