package processnice

import (
	"errors"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	for _, priority := range []int{0, 15, 19} {
		if err := Validate(priority); err != nil {
			t.Fatalf("validate(%d): %v", priority, err)
		}
	}
	for _, priority := range []int{-1, 20} {
		if err := Validate(priority); err == nil {
			t.Fatalf("validate(%d) succeeded", priority)
		}
	}
}

func TestParse(t *testing.T) {
	priority, err := Parse(" 12 ")
	if err != nil || priority != 12 {
		t.Fatal(priority, err)
	}
	for _, raw := range []string{"", "abc", "20", "-1"} {
		if _, err := Parse(raw); err == nil {
			t.Fatal(raw)
		}
	}
}

func TestResolve(t *testing.T) {
	priority, err := Resolve(nil)
	if err != nil || priority != DefaultPriority {
		t.Fatal(priority, err)
	}
	priority, err = Resolve(func(string) (string, bool) { return "", true })
	if err != nil || priority != DefaultPriority {
		t.Fatal(priority, err)
	}
	priority, err = Resolve(func(name string) (string, bool) { return " 19 ", name == EnvVar })
	if err != nil || priority != 19 {
		t.Fatal(priority, err)
	}
	if _, err = Resolve(func(name string) (string, bool) { return "bad", name == EnvVar }); err == nil {
		t.Fatal("invalid env")
	}
}

func TestApply(t *testing.T) {
	oldGet, oldSet := getPriority, setPriority
	t.Cleanup(func() {
		getPriority = oldGet
		setPriority = oldSet
	})
	called := 0
	getPriority = func() (int, error) { return 0, nil }
	setPriority = func(priority int) error {
		called++
		if priority != 12 {
			t.Fatal(priority)
		}
		return nil
	}
	if err := Apply(0); err != nil {
		t.Fatal(err)
	}
	if called != 0 {
		t.Fatal(called)
	}
	if err := Apply(12); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatal(called)
	}
	getPriority = func() (int, error) { return 15, nil }
	if err := Apply(12); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatal(called)
	}
	getPriority = func() (int, error) { return 0, errors.New("boom") }
	if err := Apply(12); err == nil || !strings.Contains(err.Error(), "get current nice priority") {
		t.Fatal(err)
	}
	getPriority = func() (int, error) { return 0, nil }
	setPriority = func(int) error { return errors.New("set") }
	if err := Apply(12); err == nil || !strings.Contains(err.Error(), "set nice priority 12") {
		t.Fatal(err)
	}
	if err := Apply(-1); err == nil {
		t.Fatal("range")
	}
}

func FuzzParse(f *testing.F) {
	for _, value := range []string{"", "0", "15", "19", "20", "-1", "abc", " 12 "} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		priority, err := Parse(value)
		if err == nil {
			if priority < 0 || priority > 19 {
				t.Fatalf("accepted priority %d", priority)
			}
			if err := Validate(priority); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestLinuxPrioritySyscalls(t *testing.T) {
	current, err := getPriorityImpl()
	if err != nil {
		t.Fatal(err)
	}
	if current < -20 || current > 19 {
		t.Fatal(current)
	}
	if err := setPriorityImpl(current); err != nil {
		if errors.Is(err, errors.ErrUnsupported) || strings.Contains(err.Error(), "operation not supported") {
			t.Skip("test binary uses cgo; release workers are CGO_ENABLED=0")
		}
		t.Fatal(err)
	}
}
