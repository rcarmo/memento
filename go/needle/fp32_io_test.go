package needle

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type failingFP32Output struct {
	name   string
	fail   string
	file   *os.File
	writes int
}

func (f *failingFP32Output) Name() string { return f.name }
func (f *failingFP32Output) Truncate(size int64) error {
	if f.fail == "truncate" {
		return errors.New("truncate")
	}
	return f.file.Truncate(size)
}
func (f *failingFP32Output) WriteAt(p []byte, off int64) (int, error) {
	f.writes++
	if (f.fail == "write" && f.writes == 1) || (f.fail == "tensor-write" && f.writes == 2) || (f.fail == "rewrite" && off == 64) {
		return 0, errors.New("write")
	}
	return f.file.WriteAt(p, off)
}
func (f *failingFP32Output) Sync() error {
	if f.fail == "sync" {
		return errors.New("sync")
	}
	return f.file.Sync()
}
func (f *failingFP32Output) Close() error {
	if f.fail == "close" {
		_ = f.file.Close()
		return errors.New("close")
	}
	return f.file.Close()
}

type failingReadFile struct{}

func (failingReadFile) Read([]byte) (int, error)   { return 0, errors.New("read") }
func (failingReadFile) Stat() (os.FileInfo, error) { return nil, errors.New("stat") }
func (failingReadFile) Close() error               { return nil }

type failingMmapFile struct{ failingReadFile }

func (failingMmapFile) Fd() uintptr { return ^uintptr(0) }

type failingSyncCloser struct{ fail string }

func (f failingSyncCloser) Sync() error {
	if f.fail == "sync" {
		return errors.New("dir sync")
	}
	return nil
}
func (f failingSyncCloser) Close() error { return nil }

func TestFP32ReadStatFailure(t *testing.T) {
	if _, err := readFP32InfoWith("ignored", func(string) (fp32ReadFile, error) { return failingReadFile{}, nil }); err == nil {
		t.Fatal("stat")
	}
	if _, err := mapReadOnlyWith("ignored", func(string) (mmapFile, error) { return failingMmapFile{}, nil }); err == nil {
		t.Fatal("mmap stat")
	}
}

func TestPrepareFP32IOFailures(t *testing.T) {
	raw := goodModel()
	dir := t.TempDir()
	source := filepath.Join(dir, "model.ndl")
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	for _, stage := range []string{"read", "mkdir", "create", "truncate", "write", "tensor-write", "rewrite", "sync", "close", "chmod", "rename", "open-dir", "dir-sync"} {
		t.Run(stage, func(t *testing.T) {
			ops := defaultFP32PrepareOps()
			switch stage {
			case "read":
				ops.readFile = func(string) ([]byte, error) { return nil, boom }
			case "mkdir":
				ops.mkdirAll = func(string, os.FileMode) error { return boom }
			case "create":
				ops.createTemp = func(string, string) (fp32Output, error) { return nil, boom }
			case "truncate", "write", "tensor-write", "rewrite", "sync", "close":
				ops.createTemp = func(directory, pattern string) (fp32Output, error) {
					file, err := os.CreateTemp(directory, pattern)
					if err != nil {
						return nil, err
					}
					return &failingFP32Output{name: file.Name(), fail: stage, file: file}, nil
				}
			case "chmod":
				ops.chmod = func(string, os.FileMode) error { return boom }
			case "rename":
				ops.rename = func(string, string) error { return boom }
			case "open-dir":
				ops.openDirectory = func(string) (fp32SyncCloser, error) { return nil, boom }
			case "dir-sync":
				ops.openDirectory = func(string) (fp32SyncCloser, error) { return failingSyncCloser{fail: "sync"}, nil }
			}
			if err := prepareFP32With(source, filepath.Join(dir, stage+".nfp32"), ops); err == nil {
				t.Fatal("expected failure")
			}
		})
	}
}
