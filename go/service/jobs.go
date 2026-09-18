package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

// Jobs composes shielded admission with per-job control/access/staging handles.
// Construction/configuration and the long-lived writer lease belong to the
// daemon. Callers must drain Workers before destroying their shared resources.
type Jobs struct {
	Controls *ProposalControls
	Identity *Identity
	DBPath   string
	Workers  Workers
}
type JobCall func(context.Context, *ProposalControls, ProposalActor) (map[string]any, SuccessOptions, error)

func (j *Jobs) Call(ctx context.Context, method string, call JobCall) (any, error) {
	// Capture only authenticated identity/session before admission. Policy is
	// resolved again on the worker handle, as MemoryService._policy does in Python.
	principal, session, err := j.Identity.requestPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	return j.Workers.Call(ctx, method, func(ctx context.Context) (any, error) {
		return j.run(ctx, principal, session, method, call, control.Connect)
	})
}
func (j *Jobs) run(ctx context.Context, principal access.Principal, session *string, method string, call JobCall, open func(context.Context, string) (*sql.DB, error)) (any, error) {
	db, err := open(ctx, j.DBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	identity, err := j.Identity.withDB(db)
	if err != nil {
		return nil, err
	}
	template := j.Controls
	codec, err := template.cursorCipher()
	if err != nil {
		return nil, err
	}
	q := template.Queue
	q.Proposals = control.Proposals{DB: db, Now: template.Queue.Proposals.Now}
	// Never copy locks. Source worker instances share only the cursor key and
	// callbacks/configuration; every worker owns its SQLite/staging handle.
	controls := &ProposalControls{Queue: q, cursor: &codec, DerivedIndexPath: template.DerivedIndexPath, MaxConceptBytes: template.MaxConceptBytes, DerivedUpdate: template.DerivedUpdate, ChangedConcepts: template.ChangedConcepts, Index: template.Index, DefaultSearchMode: template.DefaultSearchMode}
	if template.Staging != nil {
		controls.Staging = &assets.StagingStore{DB: db, Now: template.Staging.Now}
	}
	var result any
	run := func() error {
		policy, err := identity.ResolvePolicy(ctx, principal)
		if err != nil {
			return err
		}
		data, options, err := call(ctx, controls, ProposalActor{Policy: policy, MCPSessionID: session})
		if err != nil {
			return err
		}
		result, err = controls.Queue.SuccessEnvelope(data, options)
		return err
	}
	// Mirror the Python @_serialized scope, including policy resolution and
	// success revision lookup. Reconciliation and asset reads are not serialized.
	if method == "memory_operation_get" || method == "memory_proposal_asset_get" || method == "memory_read" || method == "memory_list" || method == "memory_search" || method == "memory_graph" {
		err = run()
	} else {
		err = repository.WithTransactionLock(ctx, q.Paths, run)
	}
	if err != nil {
		return FailureEnvelope(err)
	}
	return result, nil
}
func (i *Identity) withDB(db *sql.DB) (*Identity, error) {
	managed := i.managed
	if managed != nil {
		store, ok := managed.(*access.Store)
		if !ok {
			return nil, errors.New("worker identity requires a database-backed managed store")
		}
		managed = store.WithDB(db)
	}
	return &Identity{tokens: i.tokens, names: i.names, authorization: i.authorization, managed: managed, touch: i.touch}, nil
}
