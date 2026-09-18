package umcp

import (
	"errors"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	now := time.Unix(1, 0)
	dropped := []string{}
	store := newSessionStore(time.Second, 2, func(id string) { dropped = append(dropped, id) })
	store.now = func() time.Time { return now }
	next := 0
	store.token = func() (string, error) {
		next++
		if next == 1 {
			return "first", nil
		}
		return "second", nil
	}
	id, status, err := store.create("reader", "v1")
	if err != nil || status != 200 || id != "first" {
		t.Fatal(id, status, err)
	}
	for _, c := range []struct {
		ID, Principal, Version string
		Required               bool
		Status                 int
	}{{"", "reader", "v1", true, 400}, {"", "reader", "v1", false, 200}, {"missing", "reader", "v1", false, 404}, {id, "other", "v1", true, 403}, {id, "reader", "v2", true, 400}} {
		if _, got := store.validate(c.ID, c.Principal, c.Version, c.Required); got != c.Status {
			t.Fatal(c, got)
		}
	}
	session, status := store.validate(id, "reader", "v1", true)
	if status != 200 {
		t.Fatal(status)
	}
	if _, status = store.attach("missing", session); status != 404 {
		t.Fatal(status)
	}
	session.queue <- []byte("old")
	done, status := store.attach(id, session)
	if status != 200 || len(session.queue) != 0 {
		t.Fatal(status)
	}
	if _, status = store.attach(id, session); status != 409 {
		t.Fatal(status)
	}
	store.broadcast([]byte("notice"), map[string]bool{"other": true})
	if len(session.queue) != 0 {
		t.Fatal("wrong recipient")
	}
	store.broadcast([]byte("notice"), nil)
	if string(<-session.queue) != "notice" {
		t.Fatal("missing notice")
	}
	for i := 0; i < 101; i++ {
		store.broadcast([]byte("n"), nil)
	}
	if len(session.queue) != 100 {
		t.Fatal("unbounded queue")
	}
	if !store.touch(id, session) || store.touch("missing", session) {
		t.Fatal("touch")
	}
	store.detach(id, session)
	select {
	case <-done:
	default:
		t.Fatal("not disconnected")
	}
	store.broadcast([]byte("offline"), nil)
	if _, status, err = store.create("reader", "v1"); status != 200 || err != nil {
		t.Fatal(status, err)
	}
	if _, status, err = store.create("reader", "v1"); status != 503 || err != nil {
		t.Fatal(status, err)
	}
	if store.remove("missing", session) != 404 {
		t.Fatal("missing remove")
	}
	if store.remove(id, session) != 200 {
		t.Fatal("remove")
	}
	store.detach(id, session)
	now = now.Add(2 * time.Second)
	if _, status = store.validate("second", "reader", "v1", true); status != 404 {
		t.Fatal("not expired")
	}
	if len(dropped) != 2 {
		t.Fatal(dropped)
	}
	store.close()
}
func TestSessionGenerationFailures(t *testing.T) {
	s := newSessionStore(time.Hour, 2, nil)
	s.token = func() (string, error) { return "", errors.New("entropy") }
	if _, status, err := s.create("p", "v"); err == nil || status != 500 {
		t.Fatal(status, err)
	}
	s.token = func() (string, error) { return "same", nil }
	id, status, err := s.create("p", "v")
	if err != nil || status != 200 {
		t.Fatal(err)
	}
	if _, status, err = s.create("p", "v"); status != 500 || err == nil {
		t.Fatal(status, err)
	}
	session, _ := s.validate(id, "p", "", true)
	done, _ := s.attach(id, session)
	s.close()
	select {
	case <-done:
	default:
		t.Fatal("close did not disconnect")
	}
	token, err := randomSessionID()
	if err != nil || len(token) != 36 || token[14] != '4' {
		t.Fatal(token, err)
	}
}
