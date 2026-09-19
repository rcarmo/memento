package service

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/control"
	_ "modernc.org/sqlite"
)

func answerStoreTest(t *testing.T) (AnswerStore, *time.Time) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "answers.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	s := AnswerStore{DB: db, Now: func() time.Time { return now }}
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, &now
}
func answerRecord(answer string) AnswerRecord {
	return AnswerRecord{Answer: answer, AnswerSource: "deep_agent", Confidence: "high", Unresolved: []string{}, Citations: []AnswerCitation{}, ModelChain: []control.ModelAttempt{}}
}
func TestAnswerCompatibilityKeys(t *testing.T) {
	if got := NormalizeQuestion("  What\n IS  This? "); got != "what is this?" {
		t.Fatal(got)
	}
	scope := ScopeFingerprint("alice", []string{"reader", "reader"}, []string{"/work/", "/"}, []string{"/secrets/"})
	if scope != "767956eecdd3492b9ea05d9cec7204bcc5ecf8c4cf2f976b3d5f5c3abc865796" {
		t.Fatalf("scope=%s", scope)
	}
	key := ExactAnswerCacheKey("rev", "question", scope, "summary", "p1", "v1", "v1")
	if key != "59a31ad5f5e76baa1e56a04aa94123e9b2499feacf5645dd55f7994830409b80" {
		t.Fatalf("key=%s", key)
	}
}
func TestAnswerStoreExactExpiryAndLRU(t *testing.T) {
	ctx := context.Background()
	s, now := answerStoreTest(t)
	if err := s.PutExact(ctx, "one", "scope", "rev", "q", "summary", answerRecord("one"), nil, nil, 10, 1); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Second)
	if err := s.PutExact(ctx, "two", "scope", "rev", "q2", "summary", answerRecord("two"), nil, nil, 10, 1); err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetExact(ctx, "one"); err != nil || got != nil {
		t.Fatalf("one=%#v err=%v", got, err)
	}
	got, err := s.GetExact(ctx, "two")
	if err != nil || got == nil || got.AnswerSource != "exact_cache" {
		t.Fatalf("two=%#v err=%v", got, err)
	}
	*now = now.Add(11 * time.Second)
	got, err = s.GetExact(ctx, "two")
	if err != nil || got != nil {
		t.Fatalf("expired=%#v err=%v", got, err)
	}
}
func TestAnswerStoreHotIsolationInvalidationAndPruning(t *testing.T) {
	ctx := context.Background()
	s, now := answerStoreTest(t)
	if err := s.PutHotChanged(ctx, "scope", []string{"a", "b"}, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.PutHot(ctx, "scope", "q", "summary", "r1", answerRecord("r1"), []string{"a"}, 10, 1); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Second)
	if err := s.PutHot(ctx, "scope", "q", "detailed", "r1", answerRecord("detail"), []string{"b"}, 10, 1); err != nil {
		t.Fatal(err)
	}
	ids, record, err := s.GetHotContext(ctx, "scope", "q", "summary", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || (ids[0] != "a" && ids[0] != "b") || record != nil {
		t.Fatalf("ids=%v record=%#v", ids, record)
	}
	_, record, err = s.GetHotContext(ctx, "scope", "q", "detailed", "r1")
	if err != nil || record == nil || record.Answer != "detail" {
		t.Fatalf("record=%#v err=%v", record, err)
	}
	if err = s.InvalidateHot(ctx, map[string]bool{"b": true}); err != nil {
		t.Fatal(err)
	}
	_, record, err = s.GetHotContext(ctx, "scope", "q", "detailed", "r1")
	if err != nil || record != nil {
		t.Fatalf("invalidated=%#v err=%v", record, err)
	}
}
func TestAnswerStoreTraceRetention(t *testing.T) {
	ctx := context.Background()
	s, now := answerStoreTest(t)
	id := "fixed"
	r := answerRecord("answer")
	r.TraceID = &id
	result := DeepAnswerResult{Record: r, ReadConcepts: []AnswerReadConcept{{Path: "/a.md"}}, Steps: []AnswerSearchStep{{"search_knowledge", "hybrid_top_5"}}, DurationMS: 5, Usage: map[string]int{"tokens": 1}}
	got, err := s.InsertTrace(ctx, "actor", "scope", "question", "rev", result, 1, 30)
	if err != nil || got != id {
		t.Fatalf("id=%s err=%v", got, err)
	}
	*now = now.Add(time.Second)
	result.Record.TraceID = nil
	if _, err = s.InsertTrace(ctx, "actor", "scope", "other", "rev", result, 1, 30); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.DB.QueryRow("SELECT COUNT(*) FROM answer_traces").Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
