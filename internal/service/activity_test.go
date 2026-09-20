package service

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestActivityClock(t *testing.T) {
	now := time.Unix(10, 0)
	clock := NewActivityClock(func() time.Time { return now })
	now = now.Add(3 * time.Second)
	if clock.Idle() != 3*time.Second {
		t.Fatal(clock.Idle())
	}
	clock.Touch()
	if clock.Idle() != 0 {
		t.Fatal(clock.Idle())
	}
	now = now.Add(-time.Second)
	if clock.Idle() != 0 {
		t.Fatal("negative")
	}
	var nilClock *ActivityClock
	nilClock.Touch()
	if nilClock.Idle() != 0 {
		t.Fatal("nil")
	}
	clock = NewActivityClock(nil)
	if clock.Idle() < 0 {
		t.Fatal(clock.Idle())
	}
}
func TestCPUSampler(t *testing.T) {
	samples := [][]byte{[]byte("cpu  10 0 10 80 0\n"), []byte("cpu  20 0 20 90 0\n")}
	sampler := &CPUSampler{read: func(string) ([]byte, error) {
		if len(samples) == 0 {
			return nil, errors.New("read")
		}
		raw := samples[0]
		samples = samples[1:]
		return raw, nil
	}}
	if sampler.Sample() != nil {
		t.Fatal("first")
	}
	value := sampler.Sample()
	if value == nil || math.Abs(*value-66.6666667) > .001 {
		t.Fatal(value)
	}
	sampler = &CPUSampler{read: func(string) ([]byte, error) { return []byte("cpu 1 0 0 1\n"), nil }, previousIdle: 1, previousTotal: 2, initialized: true}
	if sampler.Sample() != nil {
		t.Fatal("zero")
	}
	sampler = &CPUSampler{read: func(string) ([]byte, error) { return nil, errors.New("read") }}
	if sampler.Sample() != nil {
		t.Fatal("read")
	}
	real := NewCPUSampler()
	_ = real.Sample()
}
func TestReadCPUStatFailures(t *testing.T) {
	boom := errors.New("boom")
	for _, read := range []func(string) ([]byte, error){func(string) ([]byte, error) { return nil, boom }, func(string) ([]byte, error) { return []byte("bad\n"), nil }, func(string) ([]byte, error) { return []byte("cpu x 0 0 0\n"), nil }} {
		if _, _, err := readCPUStat(read); err == nil {
			t.Fatal("error")
		}
	}
	idle, total, err := readCPUStat(func(string) ([]byte, error) { return []byte("cpu 1 2 3 4 5\n"), nil })
	if err != nil || idle != 9 || total != 15 {
		t.Fatal(idle, total, err)
	}
}
