package repository

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
)

func TestWriterLeaseReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/repository-lease.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Owner, Blocked string
		Bytes          []byte
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		path := filepath.Join(t.TempDir(), "nested", "writer.lock")
		lease, err := AcquireWriterLease(path, c.Owner)
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(content, c.Bytes) {
			t.Fatal(content, err)
		}
		if _, err = AcquireWriterLease(path, "other"); errorText(err) != c.Blocked {
			t.Fatal(err, c.Blocked)
		}
		if lease.Path != path || lease.Owner != c.Owner {
			t.Fatal(lease)
		}
		if err = lease.Release(); err != nil {
			t.Fatal(err)
		}
		if err = lease.Release(); err == nil {
			t.Fatal("double release accepted")
		}
		second, err := AcquireWriterLease(path, "second")
		if err != nil {
			t.Fatal(err)
		}
		if err = second.Release(); err != nil {
			t.Fatal(err)
		}
	}
}
func TestWriterLeaseCrossProcess(t *testing.T) {
	if path := os.Getenv("MEMENTO_GO_LEASE_TEST"); path != "" {
		lease, err := AcquireWriterLease(path, "child")
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Release()
		if _, err = os.Stdout.WriteString("locked\n"); err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, os.Stdin)
		return
	}
	path := filepath.Join(t.TempDir(), "lock")
	command := exec.Command(os.Args[0], "-test.run=^TestWriterLeaseCrossProcess$")
	command.Env = append(os.Environ(), "MEMENTO_GO_LEASE_TEST="+path)
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	signal := make([]byte, 7)
	if _, err = io.ReadFull(output, signal); err != nil || string(signal) != "locked\n" {
		t.Fatal(string(signal), err)
	}
	if _, err = AcquireWriterLease(path, "parent"); errorText(err) != "writer lease already held by child" {
		t.Fatal(err)
	}
	// Abrupt process death releases the kernel lock without deleting the file.
	if err = command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	input.Close()
	lease, err := AcquireWriterLease(path, "after-crash")
	if err != nil {
		t.Fatal(err)
	}
	if err = lease.Release(); err != nil {
		t.Fatal(err)
	}
}
func TestWriterLeaseConcurrentAcquire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	var workers sync.WaitGroup
	var mu sync.Mutex
	leases := []*WriterLease{}
	for range 20 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			lease, err := AcquireWriterLease(path, "owner")
			if err == nil {
				mu.Lock()
				leases = append(leases, lease)
				mu.Unlock()
			} else {
				var conflict *WriterLeaseError
				if !errors.As(err, &conflict) {
					t.Error(err)
				}
			}
		}()
	}
	workers.Wait()
	if len(leases) != 1 {
		t.Fatal("competing writers", len(leases))
	}
	if err := leases[0].Release(); err != nil {
		t.Fatal(err)
	}
}
func TestWriterLeaseIOFailures(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireWriterLease(filepath.Join(path, "lock"), "x"); err == nil {
		t.Fatal("mkdir")
	}
	if _, err := AcquireWriterLease(root, "x"); err == nil {
		t.Fatal("open directory")
	}
	open := func() *os.File {
		f, err := os.OpenFile(path, os.O_RDWR, 0600)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	noop := func(int, int) error { return nil }
	blocked := func(int, int) error { return syscall.EWOULDBLOCK }
	f := open()
	if _, err := acquireWriterHandle(f, path, "x", func(int, int) error { return syscall.EPERM }); !errors.Is(err, syscall.EPERM) {
		t.Fatal(err)
	}
	f = open()
	f.Close()
	if _, err := acquireWriterHandle(f, path, "x", blocked); err == nil {
		t.Fatal("seek failure")
	}
	f, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = acquireWriterHandle(f, root, "x", blocked); err == nil {
		t.Fatal("read directory")
	}
	if err = os.WriteFile(path, []byte{255}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = acquireWriterHandle(open(), path, "x", blocked); err == nil {
		t.Fatal("bad utf8")
	}
	f = open()
	f.Close()
	if _, err = acquireWriterHandle(f, path, "x", noop); err == nil {
		t.Fatal("locked seek")
	}
	f, err = os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = acquireWriterHandle(f, path, "x", noop); err == nil {
		t.Fatal("truncate")
	}
	f = open()
	if err = releaseWriterHandle(f, func(int, int) error { return syscall.EPERM }); !errors.Is(err, syscall.EPERM) {
		t.Fatal(err)
	}
	f.Close()
	// /dev/full allows seek/truncate on some platforms differently; inject a
	// closed descriptor after the flock hook to cover the deterministic path.
	f = open()
	if err = releaseWriterHandle(f, func(int, int) error { f.Close(); return nil }); err == nil {
		t.Fatal("close failure")
	}
}

type failedLeaseWrite struct{ *os.File }

func (f failedLeaseWrite) WriteString(string) (int, error) { return 0, io.ErrClosedPipe }
func TestLeaseWriteFailureReleasesLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	handle, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = acquireWriterHandle(failedLeaseWrite{handle}, path, "x", syscall.Flock); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	lease, err := AcquireWriterLease(path, "retry")
	if err != nil {
		t.Fatal("failed start stranded lock", err)
	}
	if err = lease.Release(); err != nil {
		t.Fatal(err)
	}
}
