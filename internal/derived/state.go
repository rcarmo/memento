package derived

import (
	"context"
	"fmt"
	"time"

	"github.com/rcarmo/memento/internal/access"
)

type IndexState struct {
	RepoRevision   string  `json:"repo_revision"`
	IndexRevision  string  `json:"index_revision"`
	SchemaVersion  string  `json:"schema_version"`
	Status         string  `json:"status"`
	QuarantinePath *string `json:"quarantine_path"`
}
type StatusSnapshot struct {
	State           IndexState `json:"state"`
	VisibleConcepts int        `json:"visible_concepts"`
}
type StaleIndexError struct{ RepoRevision, IndexRevision string }

func (e *StaleIndexError) Error() string {
	return fmt.Sprintf("derived index is stale: repo_revision=%s index_revision=%s", e.RepoRevision, e.IndexRevision)
}

// State and Status read an already initialised database. As with SearchLexical,
// the lifecycle owner is responsible for migration and corruption handling.
func (s ContentStore) State(ctx context.Context) (IndexState, error) { return readState(ctx, s.DB) }
func (i *Index) EmbeddingRevision(ctx context.Context) (revision string, err error) {
	err = i.withCore(ctx, false, func(s ContentStore) error {
		revision, err = requiredState(ctx, s.DB, "semantic_embedding_revision")
		return err
	})
	return revision, err
}
func readState(ctx context.Context, db executor) (IndexState, error) {
	state := IndexState{}
	for _, field := range []struct {
		key    string
		target *string
	}{{"repo_revision", &state.RepoRevision}, {"index_revision", &state.IndexRevision}, {"schema_version", &state.SchemaVersion}, {"status", &state.Status}} {
		value, err := requiredState(ctx, db, field.key)
		if err != nil {
			return IndexState{}, err
		}
		*field.target = value
	}
	var err error
	state.QuarantinePath, err = getState(ctx, db, "quarantine_path")
	return state, err
}
func (s ContentStore) Status(ctx context.Context, policy access.EffectivePolicy) (StatusSnapshot, error) {
	state, err := s.State(ctx)
	if err != nil {
		return StatusSnapshot{}, err
	}
	scope, args := authorizedPrefixes(policy, "c")
	var total int
	if err = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM concepts c WHERE "+scope, args...).Scan(&total); err != nil {
		return StatusSnapshot{}, err
	}
	return StatusSnapshot{State: state, VisibleConcepts: total}, nil
}
func (s ContentStore) SetRepoRevision(ctx context.Context, revision string) error {
	return setState(ctx, s.DB, "repo_revision", revision)
}

// WaitForFreshness checks revisions, not status: even quarantined or rebuilding
// states with equal revisions return immediately in the source. Consumers still
// enforce their own ready/quarantine checks after waiting. Cancellation is Go's
// extra explicit boundary; polling remains the source's 20 ms interval.
func (s ContentStore) WaitForFreshness(ctx context.Context, timeout time.Duration) (IndexState, error) {
	return s.waitForFreshness(ctx, timeout, time.Now, sleepFreshness)
}
func sleepFreshness(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (s ContentStore) waitForFreshness(ctx context.Context, timeout time.Duration, now func() time.Time, sleep func(context.Context, time.Duration) error) (IndexState, error) {
	return waitForState(ctx, timeout, now, sleep, s.State)
}
func waitForState(ctx context.Context, timeout time.Duration, now func() time.Time, sleep func(context.Context, time.Duration) error, read func(context.Context) (IndexState, error)) (IndexState, error) {
	deadline := now().Add(timeout)
	for {
		state, err := read(ctx)
		if err != nil {
			return IndexState{}, err
		}
		if state.IndexRevision == state.RepoRevision {
			return state, nil
		}
		if !now().Before(deadline) {
			return IndexState{}, &StaleIndexError{state.RepoRevision, state.IndexRevision}
		}
		if err = sleep(ctx, 20*time.Millisecond); err != nil {
			return IndexState{}, err
		}
	}
}
