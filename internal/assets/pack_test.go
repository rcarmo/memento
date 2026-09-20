package assets

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"os"
	"reflect"
	"testing"
)

type packFixture struct {
	Name             string
	ZIP              []byte `json:"zip"`
	Version, Kind    string
	Skill            bool
	Expected         *Manifest
	Error, ErrorType string
}

func packFixtures(t *testing.T) []packFixture {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/parity/asset-pack.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct{ Cases []packFixture }
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f.Cases
}
func TestZIPReference(t *testing.T) {
	for _, c := range packFixtures(t) {
		t.Run(c.Name, func(t *testing.T) {
			var got ValidatedPack
			var err error
			if c.Skill {
				got, err = ValidateSkillPack(c.Kind, c.Version, "# Synthetic\n", c.ZIP)
			} else {
				got, err = ValidateAssetPack(c.Kind, c.Version, c.ZIP, "", "")
			}
			if c.Error != "" {
				if err == nil || err.Error() != c.Error {
					t.Fatal(err, c.Error)
				}
			} else if err != nil || !reflect.DeepEqual(got.Manifest, *c.Expected) || !bytes.Equal(got.ZIPBytes, c.ZIP) {
				t.Fatal(got.Manifest, c.Expected, err)
			}
		})
	}
	raw, err := os.ReadFile("../../testdata/parity/asset-pack.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Versions []struct {
			Input, Error string
			Expected     [3]json.Number
		}
		Kinds []struct{ Input, Error, Expected string }
		MIME  mimeTables
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.MIME, mimeLookup) {
		t.Fatal("MIME runtime table diverged")
	}
	for _, c := range f.Versions {
		got, err := ParseStableSemver(c.Input)
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(c, err)
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
			for i, n := range got {
				want, _ := new(big.Int).SetString(string(c.Expected[i]), 10)
				if n.Cmp(want) != 0 {
					t.Fatal(got, c.Expected)
				}
			}
		}
	}
	for _, c := range f.Kinds {
		err := ValidateKind(c.Input)
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(c, err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
}
func TestZIPErrorBoundaries(t *testing.T) {
	if _, err := ValidateAssetPack("asset", "1.0.0", make([]byte, MaxZIPBytes+1), "", ""); err == nil {
		t.Fatal("encoded size")
	}
	if _, ok := decimalDigit('𞥑'); !ok {
		t.Fatal("supplementary digit")
	}
	if _, ok := decimalDigit('!'); ok {
		t.Fatal("non digit")
	}
	for _, c := range []struct{ Raw, Expected string }{{"'\n\t\r", `"'\n\t\r"`}, {"a'\"\x01", `'a\'"\x01'`}, {"a\\b", `'a\\b'`}} {
		if got := pythonPathRepr(c.Raw); got != c.Expected {
			t.Fatal(got, c.Expected)
		}
	}
	if _, err := validateMemberPath(""); err == nil {
		t.Fatal("empty")
	}
	if err := validateMetadata(0, 0, false, "x"); err != nil {
		t.Fatal(err)
	}
	if err := validateMetadata(0, 0644<<16, true, "x/"); err != nil {
		t.Fatal(err)
	}
	if err := validateRatio(100, 1, "x"); err != nil {
		t.Fatal("inclusive ratio")
	}
	// Corrupt payload and unsupported methods retain an error boundary rather
	// than returning a partial manifest as if validation had succeeded.
	for _, c := range packFixtures(t) {
		if c.Name != "stored" {
			continue
		}
		for _, kind := range []string{"crc", "method"} {
			raw := append([]byte{}, c.ZIP...)
			central := bytes.Index(raw, []byte("PK\x01\x02"))
			if kind == "method" {
				binary.LittleEndian.PutUint16(raw[central+10:], 99)
				binary.LittleEndian.PutUint16(raw[8:], 99)
			} else {
				raw[30+len("doc.md")] ^= 0xff
			}
			if _, err := ValidateAssetPack("a", "1.0.0", raw, "", ""); err == nil {
				t.Fatal(kind)
			}
		}
	}
	if err := accountMember(&Manifest{TotalUncompressedBytes: MaxUncompressedBytes}, 1, "x"); err == nil {
		t.Fatal("total limit")
	}
}
func TestMIMETypesAndTableError(t *testing.T) {
	for _, c := range []struct{ Name, Expected string }{{"unknown.zzzz", "application/octet-stream"}, {"a.txt", "text/plain"}, {"a.TXT", "text/plain"}, {"a.tgz", "application/x-tar"}, {"data:text/x-demo,hi", "text/x-demo"}, {"data:;base64,AA", "text/plain"}, {"http://host/a.txt", "text/plain"}, {"a.tar.GZ", "application/gzip"}} {
		if got := guessMediaType(c.Name); got != c.Expected {
			t.Fatal(c, got)
		}
	}
	defer func() {
		if recover() == nil {
			t.Fatal("invalid embedded table accepted")
		}
	}()
	_ = decodeMIME([]byte("{"))
}
func FuzzAssetPack(f *testing.F) {
	for _, c := range packFixturesForFuzz() {
		f.Add(c)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 65536 {
			return
		}
		_, _ = ValidateAssetPack("asset", "1.0.0", raw, "", "")
	})
}
func packFixturesForFuzz() [][]byte {
	var raw bytes.Buffer
	writer := zip.NewWriter(&raw)
	_ = writer.Close()
	return [][]byte{nil, []byte("invalid"), raw.Bytes()}
}

func TestGenericSkillAndMIMEHidden(t *testing.T) {
	empty := packFixturesForFuzz()[2]
	pack, err := ValidateAssetPack("skill", "1.0.0", empty, "", "")
	if err != nil || pack.SkillName != "skill-pack" {
		t.Fatal(pack, err)
	}
	for _, c := range []struct{ Name, Want string }{{"x.pct", "image/pict"}, {"dir/.txt", "application/octet-stream"}, {"..txt", "application/octet-stream"}} {
		if got := guessMediaType(c.Name); got != c.Want {
			t.Fatal(c, got)
		}
	}
}
