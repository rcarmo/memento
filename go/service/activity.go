package service

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ActivityClock struct {
	mu   sync.Mutex
	now  func() time.Time
	last time.Time
}

func NewActivityClock(now func() time.Time) *ActivityClock {
	if now == nil {
		now = time.Now
	}
	stamp := now()
	return &ActivityClock{now: now, last: stamp}
}
func (a *ActivityClock) Touch() {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.last = a.now()
	a.mu.Unlock()
}
func (a *ActivityClock) Idle() time.Duration {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	idle := a.now().Sub(a.last)
	if idle < 0 {
		return 0
	}
	return idle
}

type CPUSampler struct {
	mu                          sync.Mutex
	read                        func(string) ([]byte, error)
	previousIdle, previousTotal uint64
	initialized                 bool
}

func NewCPUSampler() *CPUSampler { return &CPUSampler{read: os.ReadFile} }
func (s *CPUSampler) Sample() *float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	idle, total, err := readCPUStat(s.read)
	if err != nil {
		return nil
	}
	if !s.initialized {
		s.previousIdle, s.previousTotal, s.initialized = idle, total, true
		return nil
	}
	idleDelta, totalDelta := idle-s.previousIdle, total-s.previousTotal
	s.previousIdle, s.previousTotal = idle, total
	if totalDelta == 0 {
		return nil
	}
	busy := 100 * (1 - float64(idleDelta)/float64(totalDelta))
	return &busy
}
func readCPUStat(read func(string) ([]byte, error)) (uint64, uint64, error) {
	raw, err := read("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	line, _, _ := strings.Cut(string(raw), "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, errors.New("invalid /proc/stat")
	}
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, e := strconv.ParseUint(field, 10, 64)
		if e != nil {
			return 0, 0, e
		}
		values = append(values, value)
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	total := uint64(0)
	for _, value := range values {
		total += value
	}
	return idle, total, nil
}
