package umcp

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestPythonURLRules(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/umcp-urls.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Origins []struct {
			Origin    string
			Allowed   []string
			Local     bool
			Authority string
			Valid     bool
			Error     *string
		}
		Targets []struct {
			Input    string
			Expected *struct {
				Scheme, Netloc, Path, Query, Fragment string
				RequestPath                           string `json:"request_path"`
			}
			Error *string
		}
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixtures.Origins {
		got, e := OriginIsAllowed(c.Origin, c.Allowed, c.Local, c.Authority)
		if got != c.Valid || (e == nil) != (c.Error == nil) || (e != nil && e.Error() != *c.Error) {
			t.Errorf("origin %q: %v/%v != %v/%v", c.Origin, got, e, c.Valid, c.Error)
		}
	}
	for _, c := range fixtures.Targets {
		got, e := SplitRequestTarget(c.Input)
		path, pathErr := RequestTargetPath(c.Input)
		if c.Error != nil {
			if e == nil || pathErr == nil || e.Error() != *c.Error || pathErr.Error() != *c.Error {
				t.Error(c.Input, e, pathErr, c.Error)
			}
			continue
		}
		if e != nil || pathErr != nil {
			t.Fatal(e, pathErr)
		}
		expected := RequestTarget{c.Expected.Scheme, c.Expected.Netloc, c.Expected.Path, c.Expected.Query, c.Expected.Fragment}
		if !reflect.DeepEqual(got, expected) || path != c.Expected.RequestPath {
			t.Errorf("target %q: %#v %q != %#v %q", c.Input, got, path, expected, c.Expected.RequestPath)
		}
	}
}

func FuzzURLRules(f *testing.F) {
	for _, s := range []string{"http://localhost", "http://[::1]", "http://[bad]", "http://localhost／bad", "//host/a?b#c"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) { _, _ = OriginIsAllowed(s, nil, true, ""); _, _ = RequestTargetPath(s) })
}

func TestPythonRepr(t *testing.T) {
	for _, c := range [][2]string{{"\n\r\t", `'\n\r\t'`}, {"\u200b", `'\u200b'`}, {"\U000e0001", `'\U000e0001'`}, {"b'\"c", `'b\'"c'`}, {"\x00", `'\x00'`}} {
		if got := pythonRepr(c[0]); got != c[1] {
			t.Errorf("%q: %q != %q", c[0], got, c[1])
		}
	}
}
