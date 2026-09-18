package umcp

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// Principal is authenticated transport identity. Roles/metadata are copied into
// request adapters, never supplied by a JSON tool argument.
type Principal struct {
	Name     string
	Roles    []string
	Metadata map[string]string
}

type httpSession struct {
	principal, version string
	lastSeen           time.Time
	open               bool
	queue              chan []byte
	disconnected       chan struct{}
}
type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]*httpSession
	ttl      time.Duration
	limit    int
	now      func() time.Time
	token    func() (string, error)
	drop     func(string)
}

func newSessionStore(ttl time.Duration, limit int, drop func(string)) *sessionStore {
	return &sessionStore{sessions: make(map[string]*httpSession), ttl: ttl, limit: limit, now: time.Now, token: randomSessionID, drop: drop}
}
func randomSessionID() (string, error) {
	var b [16]byte
	// Go 1.26 crypto/rand.Read fills the slice or terminates the process on
	// OS entropy failure; it no longer returns an ordinary recoverable error.
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
func (s *sessionStore) disconnect(session *httpSession) {
	if session.disconnected != nil {
		close(session.disconnected)
		session.disconnected = nil
	}
}
func (s *sessionStore) expire() {
	now := s.now()
	for id, session := range s.sessions {
		if now.Sub(session.lastSeen) >= s.ttl {
			s.disconnect(session)
			delete(s.sessions, id)
			if s.drop != nil {
				s.drop(id)
			}
		}
	}
}
func (s *sessionStore) create(principal, version string) (string, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	if len(s.sessions) >= s.limit {
		return "", 503, nil
	}
	id, err := s.token()
	if err != nil {
		return "", 500, err
	}
	if _, exists := s.sessions[id]; exists {
		return "", 500, fmt.Errorf("session ID collision")
	}
	s.sessions[id] = &httpSession{principal: principal, version: version, lastSeen: s.now(), queue: make(chan []byte, 100)}
	return id, 200, nil
}
func (s *sessionStore) validate(id, principal, version string, required bool) (*httpSession, int) {
	if id == "" {
		if required {
			return nil, 400
		}
		return nil, 200
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	session := s.sessions[id]
	if session == nil {
		return nil, 404
	}
	if session.principal != principal {
		return nil, 403
	}
	if version != "" && version != session.version {
		return nil, 400
	}
	session.lastSeen = s.now()
	return session, 200
}
func (s *sessionStore) remove(id string, session *httpSession) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[id] != session || session == nil {
		return 404
	}
	s.disconnect(session)
	delete(s.sessions, id)
	if s.drop != nil {
		s.drop(id)
	}
	return 200
}
func (s *sessionStore) attach(id string, session *httpSession) (<-chan struct{}, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[id] != session || session == nil {
		return nil, 404
	}
	if session.open {
		return nil, 409
	}
	for len(session.queue) > 0 {
		<-session.queue
	}
	session.disconnected = make(chan struct{})
	session.open = true
	return session.disconnected, 200
}
func (s *sessionStore) detach(id string, session *httpSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[id] == session {
		s.disconnect(session)
		session.open = false
		session.lastSeen = s.now()
	}
}
func (s *sessionStore) touch(id string, session *httpSession) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[id] != session {
		return false
	}
	session.lastSeen = s.now()
	return true
}
func (s *sessionStore) broadcast(payload []byte, recipients map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	for id, session := range s.sessions {
		if !session.open || recipients != nil && !recipients[id] {
			continue
		}
		copy := append([]byte(nil), payload...)
		select {
		case session.queue <- copy:
		default:
		}
	}
}
func (s *sessionStore) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, session := range s.sessions {
		s.disconnect(session)
		delete(s.sessions, id)
		if s.drop != nil {
			s.drop(id)
		}
	}
}
