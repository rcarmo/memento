package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestAssetRetrievalReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/asset-retrieval.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		ZIP            []byte `json:"zip"`
		Manifest       Manifest
		ManifestDigest string `json:"manifest_digest"`
		Limits         map[string]int
		Cases          []struct {
			Path          string
			Offset, Limit int64
			Error         string
			Expected      map[string]any
		}
		Invalid []struct {
			Manifest      Manifest
			Digest, Error string
		} `json:"invalid_manifests"`
		Bad []struct {
			Name  string
			ZIP   []byte `json:"zip"`
			Error string
		} `json:"bad_zips"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(RetrievalLimits(), fixture.Limits) {
		t.Fatal("limits")
	}
	if got := ManifestDigest(fixture.Manifest); got != fixture.ManifestDigest {
		t.Fatal(got, fixture.ManifestDigest)
	}
	if err := CheckedManifest(fixture.Manifest, fixture.Manifest.SHA256); err != nil {
		t.Fatal(err)
	}
	for i, c := range fixture.Cases {
		got, err := ReadPackFile(bytes.NewReader(fixture.ZIP), int64(len(fixture.ZIP)), fixture.Manifest, c.Path, c.Offset, c.Limit)
		want := c.Error
		if c.Path == "missing" {
			want = "asset file is not declared in the manifest"
		}
		if want != "" {
			if err == nil || err.Error() != want {
				t.Fatal(i, err, want)
			}
		} else if err != nil || !reflect.DeepEqual(normal(got), c.Expected) {
			t.Fatal(i, got, c.Expected, err)
		}
	}
	for _, c := range fixture.Invalid {
		if err := CheckedManifest(c.Manifest, c.Digest); err == nil || err.Error() != c.Error {
			t.Fatal(c, err)
		}
	}
	for _, c := range fixture.Bad {
		if _, err := ReadPackFile(bytes.NewReader(c.ZIP), int64(len(c.ZIP)), fixture.Manifest, "dir/text.md", 0, 10); err == nil || err.Error() != c.Error {
			t.Fatal(c.Name, err, c.Error)
		}
	}
}

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestVerifiedSlicesAndBoundaries(t *testing.T) {
	data := bytes.Repeat([]byte("abcdef"), 25000)
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	for _, offset := range []int64{0, 1, 65535, 65536, int64(len(data))} {
		got, err := VerifiedSlice(bytes.NewReader(data), int64(len(data)), digest, offset, 10)
		end := min(offset+10, int64(len(data)))
		if err != nil || !bytes.Equal(got, data[offset:end]) {
			t.Fatal(offset, err)
		}
	}
	for _, c := range []struct {
		Raw                  []byte
		Total, Offset, Limit int64
		Digest               string
	}{{data, 1, 0, 1, digest}, {data, int64(len(data)) + 1, 0, 1, digest}, {data, int64(len(data)), 0, 1, "wrong"}, {data, int64(len(data)), -1, 1, digest}} {
		if _, err := VerifiedSlice(bytes.NewReader(c.Raw), c.Total, c.Digest, c.Offset, c.Limit); err == nil {
			t.Fatal(c.Total, c.Offset)
		}
	}
	if _, err := VerifiedSlice(failedReader{}, 0, digest, 0, 1); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err := ValidateRange(0, 1, nil); err != nil {
		t.Fatal(err)
	}
	if err := CheckExpectedDigest(nil, digest); err != nil {
		t.Fatal(err)
	}
	if err := CheckExpectedDigest(&digest, digest); err != nil {
		t.Fatal(err)
	}
	bad := "BAD"
	if err := CheckExpectedDigest(&bad, digest); err == nil {
		t.Fatal("bad digest")
	}
	for _, size := range []int64{0, -1, MaxZIPBytes + 1} {
		if err := ValidateArchiveSize(size); err == nil {
			t.Fatal(size)
		}
	}
	if err := ValidateArchiveSize(1); err != nil {
		t.Fatal(err)
	}
	for _, total := range []uint64{MaxUncompressedBytes + 1} {
		manifest := Manifest{SHA256: digest, FileCount: 4, TotalUncompressedBytes: total, Entries: []ManifestEntry{}}
		for i := range 4 {
			manifest.Entries = append(manifest.Entries, ManifestEntry{Path: fmt.Sprint(i), Size: MaxFileBytes, SHA256: digest, MediaType: "x"})
		}
		if err := CheckedManifest(manifest, digest); err == nil {
			t.Fatal("total overflow")
		}
	}
}

func TestBzipRetrievalAndZeroCRC(t *testing.T) {
	for _, c := range packFixtures(t) {
		if c.Name != "method-12" {
			continue
		}
		pack, err := ValidateSkillPack("demo", "1.0.0", "# Synthetic\n", c.ZIP)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ReadPackFile(bytes.NewReader(c.ZIP), int64(len(c.ZIP)), pack.Manifest, "SKILL.md", 0, 100)
		if err != nil || got["content"] != "# Synthetic\n" {
			t.Fatal(got, err)
		}
		raw := append([]byte{}, c.ZIP...)
		position := bytes.Index(raw, []byte("PK\x01\x02"))
		binary.LittleEndian.PutUint32(raw[position+16:], 0)
		if _, err = ValidateSkillPack("demo", "1.0.0", "# Synthetic\n", raw); err == nil {
			t.Fatal("zero CRC accepted")
		}
	}
	// Test preserved asset digest error separately from decoder/CRC failures.
	raw := syntheticZIP(t)
	pack, err := ValidateAssetPack("a", "1.0.0", raw, "", "")
	if err != nil {
		t.Fatal(err)
	}
	pack.Manifest.Entries[0].SHA256 = strings.Repeat("0", 64)
	if _, err = ReadPackFile(bytes.NewReader(raw), int64(len(raw)), pack.Manifest, pack.Manifest.Entries[0].Path, 0, 10); err == nil || err.Error() != "asset digest/size mismatch" {
		t.Fatal(err)
	}
}

func TestAssetReadNonzeroCRCMismatch(t *testing.T) {
	raw := syntheticZIP(t)
	pack, err := ValidateAssetPack("asset", "1.0.0", raw, "", "")
	if err != nil {
		t.Fatal(err)
	}
	position := bytes.Index(raw, []byte("PK\x01\x02"))
	binary.LittleEndian.PutUint32(raw[position+16:], 1)
	if _, err = ReadPackFile(bytes.NewReader(raw), int64(len(raw)), pack.Manifest, pack.Manifest.Entries[0].Path, 0, 10); err == nil || err.Error() != "asset ZIP validation failed" {
		t.Fatal(err)
	}
}

func FuzzAssetRetrieval(f *testing.F) {
	for _, raw := range packFixturesForFuzz() {
		f.Add(raw, "file", int64(0), int64(10))
	}
	f.Fuzz(func(t *testing.T, raw []byte, path string, offset, limit int64) {
		if len(raw) > 65536 || len(path) > 4096 {
			return
		}
		manifest := Manifest{Entries: []ManifestEntry{{Path: path, Size: uint64(len(raw)), SHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), MediaType: "application/octet-stream"}}, FileCount: 1, TotalUncompressedBytes: uint64(len(raw)), SHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
		_, _ = ReadPackFile(bytes.NewReader(raw), int64(len(raw)), manifest, path, offset, limit)
		_, _ = VerifiedSlice(bytes.NewReader(raw), int64(len(raw)), manifest.SHA256, offset, limit)
	})
}
