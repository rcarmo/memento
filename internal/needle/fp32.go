package needle

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"unicode/utf8"
	"unsafe"
)

const (
	fp32Magic       = "NFP32LE\x00"
	fp32Version     = uint32(1)
	fp32FixedSize   = 96
	fp32MaxHeader   = 16 << 20
	fp32MaxFileSize = 4 << 30
	fp32DataAlign   = 4096
	fp32TensorAlign = 64
)

type fp32TensorRecord struct {
	name   string
	shape  []uint32
	offset uint64
	count  uint64
}

// FP32Info identifies an architecture-independent, little-endian mapped model.
type FP32Info struct {
	Config       Config
	SourceSHA256 [32]byte
	DataSHA256   [32]byte
	TensorCount  int
	FileSize     int64
}

// MappedFP32Model owns one read-only mapping. Router tensor slices point into it
// and remain valid until Close returns.
type MappedFP32Model struct {
	config       Config
	sourceSHA256 [32]byte
	tensors      map[string][]float32
	shapes       map[string][]uint32
	mapping      []byte
	closeOnce    sync.Once
	closeErr     error
}

func alignUp(value, alignment uint64) uint64 {
	return (value + alignment - 1) &^ (alignment - 1)
}

type fp32Output interface {
	Name() string
	Truncate(int64) error
	WriteAt([]byte, int64) (int, error)
	Sync() error
	Close() error
}
type fp32SyncCloser interface {
	Sync() error
	Close() error
}
type fp32ReadFile interface {
	io.Reader
	Stat() (os.FileInfo, error)
	Close() error
}
type fp32PrepareOps struct {
	readFile      func(string) ([]byte, error)
	mkdirAll      func(string, os.FileMode) error
	createTemp    func(string, string) (fp32Output, error)
	remove        func(string) error
	chmod         func(string, os.FileMode) error
	rename        func(string, string) error
	openDirectory func(string) (fp32SyncCloser, error)
}

func defaultFP32PrepareOps() fp32PrepareOps {
	return fp32PrepareOps{os.ReadFile, os.MkdirAll, func(dir, pattern string) (fp32Output, error) { return os.CreateTemp(dir, pattern) }, os.Remove, os.Chmod, os.Rename, func(path string) (fp32SyncCloser, error) { return os.Open(path) }}
}

// PrepareFP32 converts a validated NDL1 model into a deterministic mmapable
// FP32 sidecar. The destination is atomically replaced only after fsync.
func PrepareFP32(sourcePath, destinationPath string) error {
	return prepareFP32With(sourcePath, destinationPath, defaultFP32PrepareOps())
}
func prepareFP32With(sourcePath, destinationPath string, ops fp32PrepareOps) error {
	raw, err := ops.readFile(sourcePath)
	if err != nil {
		return err
	}
	model, err := FromBytes(raw)
	if err != nil {
		return err
	}
	return prepareFP32Model(model, sha256.Sum256(raw), destinationPath, ops)
}
func prepareFP32Model(model *Model, sourceDigest [32]byte, destinationPath string, ops fp32PrepareOps) error {
	config, _ := json.Marshal(model.config)
	names := model.TensorNames()
	records := make([]fp32TensorRecord, 0, len(names))
	headerLength := uint64(fp32FixedSize + len(config))
	for _, name := range names {
		tensor := model.tensors[name]
		headerLength += uint64(24 + len(name) + len(tensor.shape)*4)
	}
	dataOffset := alignUp(headerLength, fp32DataAlign)
	next := dataOffset
	for _, name := range names {
		tensor := model.tensors[name]
		next = alignUp(next, fp32TensorAlign)
		records = append(records, fp32TensorRecord{name: name, shape: append([]uint32{}, tensor.shape...), offset: next, count: uint64(len(tensor.data) / 2)})
		next += uint64(len(tensor.data) / 2 * 4)
	}
	if err := ops.mkdirAll(filepath.Dir(destinationPath), 0755); err != nil {
		return err
	}
	temporary, err := ops.createTemp(filepath.Dir(destinationPath), ".memento-needle-fp32-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = ops.remove(temporaryPath)
		}
	}()
	if err = temporary.Truncate(int64(next)); err != nil {
		return err
	}
	header := make([]byte, dataOffset)
	copy(header[:8], fp32Magic)
	binary.LittleEndian.PutUint32(header[8:12], fp32Version)
	binary.LittleEndian.PutUint32(header[12:16], uint32(headerLength))
	binary.LittleEndian.PutUint64(header[16:24], dataOffset)
	binary.LittleEndian.PutUint32(header[24:28], uint32(len(records)))
	binary.LittleEndian.PutUint32(header[28:32], uint32(len(config)))
	copy(header[32:64], sourceDigest[:])
	dataDigest := sha256.New()
	cursor := fp32FixedSize
	copy(header[cursor:], config)
	cursor += len(config)
	for _, record := range records {
		binary.LittleEndian.PutUint16(header[cursor:cursor+2], uint16(len(record.name)))
		binary.LittleEndian.PutUint16(header[cursor+2:cursor+4], uint16(len(record.shape)))
		binary.LittleEndian.PutUint64(header[cursor+8:cursor+16], record.offset)
		binary.LittleEndian.PutUint64(header[cursor+16:cursor+24], record.count)
		cursor += 24
		copy(header[cursor:], record.name)
		cursor += len(record.name)
		for _, dimension := range record.shape {
			binary.LittleEndian.PutUint32(header[cursor:cursor+4], dimension)
			cursor += 4
		}
	}
	if _, err = temporary.WriteAt(header, 0); err != nil {
		return err
	}
	buffer := make([]byte, 1<<20)
	for _, record := range records {
		tensor := model.tensors[record.name]
		written := uint64(0)
		for base := 0; base < len(tensor.data); {
			pairs := min((len(tensor.data)-base)/2, len(buffer)/4)
			for item := 0; item < pairs; item++ {
				bits := uint32(binary.LittleEndian.Uint16(tensor.data[base+item*2:])) << 16
				binary.LittleEndian.PutUint32(buffer[item*4:], bits)
			}
			chunk := buffer[:pairs*4]
			if _, err = temporary.WriteAt(chunk, int64(record.offset+written)); err != nil {
				return err
			}
			_, _ = dataDigest.Write(chunk)
			base += pairs * 2
			written += uint64(len(chunk))
		}
	}
	copy(header[64:96], dataDigest.Sum(nil))
	if _, err = temporary.WriteAt(header[64:96], 64); err != nil {
		return err
	}
	if err = temporary.Sync(); err != nil {
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = ops.chmod(temporaryPath, 0644); err != nil {
		return err
	}
	if err = ops.rename(temporaryPath, destinationPath); err != nil {
		return err
	}
	keep = true
	directory, err := ops.openDirectory(filepath.Dir(destinationPath))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

// parseFP32Header is kept separate from I/O so malformed format boundaries are
// exhaustively fuzzed and unit-tested without constructing large files.
func parseFP32Header(data []byte, fileSize int64) (FP32Info, []fp32TensorRecord, error) {
	return readFP32Header(data, fileSize)
}
func readFP32Header(data []byte, fileSize int64) (FP32Info, []fp32TensorRecord, error) {
	var info FP32Info
	if len(data) < fp32FixedSize || string(data[:8]) != fp32Magic {
		return info, nil, errors.New("invalid FP32 model magic")
	}
	if version := binary.LittleEndian.Uint32(data[8:12]); version != fp32Version {
		return info, nil, fmt.Errorf("unsupported FP32 model version: %d", version)
	}
	headerLength := uint64(binary.LittleEndian.Uint32(data[12:16]))
	dataOffset := binary.LittleEndian.Uint64(data[16:24])
	count := uint64(binary.LittleEndian.Uint32(data[24:28]))
	configLength := uint64(binary.LittleEndian.Uint32(data[28:32]))
	if fileSize < fp32FixedSize || fileSize > fp32MaxFileSize || headerLength < fp32FixedSize || headerLength > fp32MaxHeader || headerLength > dataOffset || dataOffset > uint64(fileSize) || dataOffset%fp32DataAlign != 0 || headerLength > uint64(len(data)) {
		return info, nil, errors.New("invalid FP32 model header bounds")
	}
	if configLength > headerLength-fp32FixedSize {
		return info, nil, errors.New("invalid FP32 model config length")
	}
	if err := json.Unmarshal(data[fp32FixedSize:fp32FixedSize+configLength], &info.Config); err != nil {
		return info, nil, fmt.Errorf("invalid FP32 model config: %w", err)
	}
	copy(info.SourceSHA256[:], data[32:64])
	copy(info.DataSHA256[:], data[64:96])
	info.TensorCount, info.FileSize = int(count), fileSize
	cursor := uint64(fp32FixedSize) + configLength
	if count > (headerLength-cursor)/24 {
		return info, nil, errors.New("invalid FP32 tensor count")
	}
	records := make([]fp32TensorRecord, 0, count)
	seen := make(map[string]struct{}, count)
	for index := uint64(0); index < count; index++ {
		nameLength := uint64(binary.LittleEndian.Uint16(data[cursor : cursor+2]))
		rank := uint64(binary.LittleEndian.Uint16(data[cursor+2 : cursor+4]))
		offset := binary.LittleEndian.Uint64(data[cursor+8 : cursor+16])
		elements := binary.LittleEndian.Uint64(data[cursor+16 : cursor+24])
		cursor += 24
		if nameLength == 0 || rank > 16 || nameLength+rank*4 > headerLength-cursor {
			return info, nil, errors.New("invalid FP32 tensor descriptor")
		}
		nameBytes := data[cursor : cursor+nameLength]
		if !utf8.Valid(nameBytes) {
			return info, nil, errors.New("invalid FP32 tensor name")
		}
		name := string(nameBytes)
		cursor += nameLength
		shape := make([]uint32, rank)
		product := uint64(1)
		for dimension := range shape {
			shape[dimension] = binary.LittleEndian.Uint32(data[cursor : cursor+4])
			cursor += 4
			if shape[dimension] == 0 || product > math.MaxUint64/uint64(shape[dimension]) {
				return info, nil, errors.New("invalid FP32 tensor shape")
			}
			product *= uint64(shape[dimension])
		}
		if product != elements || offset < dataOffset || offset > uint64(fileSize) || offset%4 != 0 || elements > (uint64(fileSize)-offset)/4 {
			return info, nil, fmt.Errorf("invalid FP32 tensor bounds for %s", name)
		}
		if _, duplicate := seen[name]; duplicate {
			return info, nil, fmt.Errorf("duplicate FP32 tensor %s", name)
		}
		seen[name] = struct{}{}
		records = append(records, fp32TensorRecord{name: name, shape: shape, offset: offset, count: elements})
	}
	if cursor != headerLength {
		return info, nil, errors.New("trailing FP32 tensor directory bytes")
	}
	ordered := append([]fp32TensorRecord{}, records...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].offset < ordered[j].offset })
	for index := 1; index < len(ordered); index++ {
		previousEnd := ordered[index-1].offset + ordered[index-1].count*4
		if ordered[index].offset < previousEnd {
			return info, nil, errors.New("overlapping FP32 tensors")
		}
	}
	return info, records, nil
}

// ReadFP32Info validates the bounded header without mapping or touching weights.
func ReadFP32Info(path string) (FP32Info, error) {
	return readFP32InfoWith(path, func(path string) (fp32ReadFile, error) { return os.Open(path) })
}
func readFP32InfoWith(path string, open func(string) (fp32ReadFile, error)) (FP32Info, error) {
	file, err := open(path)
	if err != nil {
		return FP32Info{}, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return FP32Info{}, err
	}
	return readFP32InfoFrom(file, stat.Size())
}
func readFP32InfoFrom(file io.Reader, size int64) (FP32Info, error) {
	fixed := make([]byte, fp32FixedSize)
	if _, err := io.ReadFull(file, fixed); err != nil {
		return FP32Info{}, err
	}
	headerLength := binary.LittleEndian.Uint32(fixed[12:16])
	if headerLength < fp32FixedSize || headerLength > fp32MaxHeader || int64(headerLength) > size {
		return FP32Info{}, errors.New("invalid FP32 model header length")
	}
	header := make([]byte, headerLength)
	copy(header, fixed)
	if _, err := io.ReadFull(file, header[fp32FixedSize:]); err != nil {
		return FP32Info{}, err
	}
	info, _, err := readFP32Header(header, size)
	return info, err
}

// VerifyFP32 hashes the logical tensor bytes. Packaging and deployment checks
// use it once; latency-sensitive workers rely on the immutable image plus the
// bounded structural validation performed by LoadMappedFP32.
func VerifyFP32(path string) error { return verifyFP32With(path, mapReadOnly) }
func verifyFP32With(path string, mmap func(string) ([]byte, error)) error {
	mapping, err := mmap(path)
	if err != nil {
		return err
	}
	defer unmapReadOnly(mapping)
	info, records, err := readFP32Header(mapping, int64(len(mapping)))
	if err != nil {
		return err
	}
	digest := sha256.New()
	for _, record := range records {
		start, end := record.offset, record.offset+record.count*4
		_, _ = digest.Write(mapping[start:end])
	}
	if subtle.ConstantTimeCompare(digest.Sum(nil), info.DataSHA256[:]) != 1 {
		return errors.New("FP32 model data checksum mismatch")
	}
	return nil
}

// LoadMappedFP32 maps a prepared model and exposes tensor slices without copies.
func LoadMappedFP32(path string) (*MappedFP32Model, error) {
	mapping, err := mapReadOnly(path)
	if err != nil {
		return nil, err
	}
	info, records, err := readFP32Header(mapping, int64(len(mapping)))
	if err != nil {
		_ = unmapReadOnly(mapping)
		return nil, err
	}
	model := &MappedFP32Model{config: info.Config, sourceSHA256: info.SourceSHA256, tensors: make(map[string][]float32, len(records)), shapes: make(map[string][]uint32, len(records)), mapping: mapping}
	for _, record := range records {
		pointer := unsafe.Pointer(&mapping[record.offset])
		model.tensors[record.name] = unsafe.Slice((*float32)(pointer), int(record.count))
		model.shapes[record.name] = record.shape
	}
	return model, nil
}

func (m *MappedFP32Model) Config() Config         { return m.config }
func (m *MappedFP32Model) SourceSHA256() [32]byte { return m.sourceSHA256 }
func (m *MappedFP32Model) TensorNames() []string {
	names := make([]string, 0, len(m.tensors))
	for name := range m.tensors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func (m *MappedFP32Model) tensor(name string, shape []uint32) ([]float32, error) {
	values, ok := m.tensors[name]
	if !ok {
		return nil, fmt.Errorf("missing tensor: %s", name)
	}
	if !equalShape(m.shapes[name], shape) {
		return nil, fmt.Errorf("invalid tensor shape for %s: expected %v, got %v", name, shape, m.shapes[name])
	}
	return values, nil
}
func (m *MappedFP32Model) Close() error {
	m.closeOnce.Do(func() {
		m.tensors = nil
		m.shapes = nil
		m.closeErr = unmapReadOnly(m.mapping)
		m.mapping = nil
	})
	return m.closeErr
}

// MappedRouter keeps its backing mapping alive until Close.
type MappedRouter struct {
	*Router
	model *MappedFP32Model
}

func LoadMappedRouter(path string) (*MappedRouter, error) {
	model, err := LoadMappedFP32(path)
	if err != nil {
		return nil, err
	}
	router, err := newRouter(model.config, model.tensor)
	if err != nil {
		_ = model.Close()
		return nil, err
	}
	return &MappedRouter{Router: router, model: model}, nil
}
func (r *MappedRouter) Close() error {
	if r == nil || r.model == nil {
		return nil
	}
	return r.model.Close()
}
