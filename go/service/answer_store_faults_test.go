package service

import (
	"context"
	"testing"
)

func abortTrigger(t *testing.T, s AnswerStore, name, table, event string) {
	t.Helper()
	_, err := s.DB.Exec(`CREATE TRIGGER ` + name + ` BEFORE ` + event + ` ON ` + table + ` BEGIN SELECT RAISE(ABORT,'boom'); END`)
	if err != nil {
		t.Fatal(err)
	}
}
func TestAnswerStoreSQLFaults(t *testing.T) {
	ctx := context.Background()
	t.Run("exact insert", func(t *testing.T) {
		s, _ := answerStoreTest(t)
		abortTrigger(t, s, "x", "answer_cache", "INSERT")
		if err := s.PutExact(ctx, "k", "s", "r", "q", "m", answerRecord("a"), nil, nil, 1, 1); err == nil {
			t.Fatal("insert")
		}
	})
	t.Run("exact prune", func(t *testing.T) {
		s, now := answerStoreTest(t)
		if err := s.PutExact(ctx, "k", "s", "r", "q", "m", answerRecord("a"), nil, nil, 1, 2); err != nil {
			t.Fatal(err)
		}
		*now = now.Add(2e9)
		abortTrigger(t, s, "x", "answer_cache", "DELETE")
		if err := s.PutExact(ctx, "k2", "s", "r", "q", "m", answerRecord("a"), nil, nil, 1, 2); err == nil {
			t.Fatal("prune")
		}
	})
	t.Run("exact access update", func(t *testing.T) {
		s, _ := answerStoreTest(t)
		if err := s.PutExact(ctx, "k", "s", "r", "q", "m", answerRecord("a"), nil, nil, 10, 1); err != nil {
			t.Fatal(err)
		}
		abortTrigger(t, s, "x", "answer_cache", "UPDATE")
		if _, err := s.GetExact(ctx, "k"); err == nil {
			t.Fatal("update")
		}
	})
	t.Run("exact expiry delete", func(t *testing.T) {
		s, now := answerStoreTest(t)
		if err := s.PutExact(ctx, "k", "s", "r", "q", "m", answerRecord("a"), nil, nil, 1, 1); err != nil {
			t.Fatal(err)
		}
		*now = now.Add(2e9)
		abortTrigger(t, s, "x", "answer_cache", "DELETE")
		if _, err := s.GetExact(ctx, "k"); err == nil {
			t.Fatal("delete")
		}
	})
	t.Run("hot cleanup", func(t *testing.T) {
		s, _ := answerStoreTest(t)
		_, _ = s.DB.Exec(`INSERT INTO hot_changed_concepts VALUES('s','x','2010')`)
		abortTrigger(t, s, "x", "hot_changed_concepts", "DELETE")
		if _, _, err := s.GetHotContext(ctx, "s", "q", "m", "r"); err == nil {
			t.Fatal("cleanup")
		}
	})
	t.Run("hot malformed", func(t *testing.T) {
		s, _ := answerStoreTest(t)
		_, _ = s.DB.Exec(`INSERT INTO hot_answers VALUES('s','` + answerHash("q") + `','q','m','r','{','[]','2026-01-01T00:00:00Z','2099-01-01T00:00:00Z')`)
		if _, _, err := s.GetHotContext(ctx, "s", "q", "m", "r"); err == nil {
			t.Fatal("json")
		}
	})
	t.Run("changed insert", func(t *testing.T) {
		s, _ := answerStoreTest(t)
		abortTrigger(t, s, "x", "hot_changed_concepts", "INSERT")
		if err := s.PutHotChanged(ctx, "s", []string{"x"}, 1); err == nil {
			t.Fatal("insert")
		}
	})
	t.Run("changed prune", func(t *testing.T) {
		s, _ := answerStoreTest(t)
		if err := s.PutHotChanged(ctx, "s", []string{"a", "b"}, 2); err != nil {
			t.Fatal(err)
		}
		abortTrigger(t, s, "x", "hot_changed_concepts", "DELETE")
		if err := s.PutHotChanged(ctx, "s", []string{"c"}, 1); err == nil {
			t.Fatal("prune")
		}
	})
	t.Run("hot insert", func(t *testing.T) {
		s, _ := answerStoreTest(t)
		abortTrigger(t, s, "x", "hot_answers", "INSERT")
		if err := s.PutHot(ctx, "s", "q", "m", "r", answerRecord("a"), nil, 1, 1); err == nil {
			t.Fatal("insert")
		}
	})
	t.Run("hot prune", func(t *testing.T) {
		s, _ := answerStoreTest(t)
		if err := s.PutHot(ctx, "s", "a", "m", "r", answerRecord("a"), nil, 10, 2); err != nil {
			t.Fatal(err)
		}
		abortTrigger(t, s, "x", "hot_answers", "DELETE")
		if err := s.PutHot(ctx, "s", "b", "m", "r", answerRecord("b"), nil, 10, 1); err == nil {
			t.Fatal("prune")
		}
	})
	t.Run("invalidate json", func(t *testing.T) {
		s, _ := answerStoreTest(t)
		_, _ = s.DB.Exec(`INSERT INTO hot_answers VALUES('s','h','q','m','r','{}','{','2026','2099')`)
		if err := s.InvalidateHot(ctx, map[string]bool{"x": true}); err == nil {
			t.Fatal("json")
		}
	})
	t.Run("invalidate delete", func(t *testing.T) {
		s, _ := answerStoreTest(t)
		if err := s.PutHot(ctx, "s", "q", "m", "r", answerRecord("a"), []string{"x"}, 10, 1); err != nil {
			t.Fatal(err)
		}
		abortTrigger(t, s, "x", "hot_answers", "DELETE")
		if err := s.InvalidateHot(ctx, map[string]bool{"x": true}); err == nil {
			t.Fatal("delete")
		}
	})
	t.Run("trace insert", func(t *testing.T) {
		s, _ := answerStoreTest(t)
		abortTrigger(t, s, "x", "answer_traces", "INSERT")
		if _, err := s.InsertTrace(ctx, "p", "s", "q", "r", DeepAnswerResult{Record: answerRecord("a")}, 1, 1); err == nil {
			t.Fatal("insert")
		}
	})
}
