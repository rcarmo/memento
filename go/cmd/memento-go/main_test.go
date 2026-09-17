package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	for _, args := range [][]string{nil, {"serve"}, {"unknown"}, {"version", "extra"}, {"version"}} {
		var out, err bytes.Buffer
		code := run(args, &out, &err)
		if len(args) == 1 && args[0] == "version" {
			if code != 0 || !strings.Contains(out.String(), "server not implemented") || err.Len() != 0 {
				t.Fatal(code, out.String(), err.String())
			}
		} else if code != 2 || out.Len() != 0 || !strings.Contains(err.String(), "not implemented") {
			t.Fatal(code, out.String(), err.String())
		}
	}
}

func TestMainWiring(t *testing.T) {
	oldArgs, oldExit := os.Args, exit
	t.Cleanup(func() { os.Args = oldArgs; exit = oldExit })
	os.Args = []string{"memento-go", "version"}
	called := false
	exit = func(code int) {
		called = true
		if code != 0 {
			t.Fatal(code)
		}
	}
	main()
	if !called {
		t.Fatal("main did not terminate with run status")
	}
}
