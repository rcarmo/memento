package processnice

import (
	"errors"
	"strings"
	"syscall"
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
	oldGet, oldAll := getPrioritySyscall, allThreadsSyscall
	t.Cleanup(func() {
		getPrioritySyscall = oldGet
		allThreadsSyscall = oldAll
	})
	getPrioritySyscall = func(int, int) (int, error) { return 5, nil }
	if current, err := getPriorityImpl(); err != nil || current != 15 {
		t.Fatal(current, err)
	}
	getPrioritySyscall = func(int, int) (int, error) { return 0, errors.New("get") }
	if _, err := getPriorityImpl(); err == nil {
		t.Fatal("get error")
	}
	var trap, class, who, priority uintptr
	allThreadsSyscall = func(a, b, c, d uintptr) (uintptr, uintptr, syscall.Errno) {
		trap, class, who, priority = a, b, c, d
		return 0, 0, 0
	}
	if err := setPriorityImpl(19); err != nil || trap != syscall.SYS_SETPRIORITY || class != syscall.PRIO_PROCESS || who != 0 || priority != 19 {
		t.Fatal(err, trap, class, who, priority)
	}
	allThreadsSyscall = func(uintptr, uintptr, uintptr, uintptr) (uintptr, uintptr, syscall.Errno) {
		return 0, 0, syscall.EPERM
	}
	if err := setPriorityImpl(19); !errors.Is(err, syscall.EPERM) {
		t.Fatal(err)
	}
}
