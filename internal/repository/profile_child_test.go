package repository

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
)

// A deliberate SIGKILL cannot flush profiles. Capture the exercised child
// workload at the rendezvous immediately before the parent kills it, preserving
// the actual abrupt-termination recovery test (no graceful-exit substitute).
func checkpointChildProfiles(t *testing.T) {
	t.Helper()
	directory := os.Getenv("MEMENTO_TEST_PROFILE_PROCESS_DIR")
	if directory == "" {
		return
	}
	pprof.StopCPUProfile()
	runtime.GC()
	file, err := os.Create(filepath.Join(directory, "heap.pprof"))
	if err != nil {
		t.Fatal(err)
	}
	err = pprof.Lookup("allocs").WriteTo(file, 0)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	if err = os.WriteFile(filepath.Join(directory, "checkpoint.txt"), []byte("Capture immediately before deliberate process kill; no post-kill sampling possible.\n"), 0600); err != nil {
		t.Fatal(err)
	}
}
