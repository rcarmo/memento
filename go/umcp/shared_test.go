package umcp

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestPythonSharedParity(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/umcp-shared.json")
	if err != nil {
		t.Fatal(err)
	}
	type validCase struct {
		Input json.RawMessage
		Valid bool
	}
	var fixture struct {
		IDs       []validCase
		Responses []validCase
		Versions  []struct {
			Accepted   *string
			Preferred  string
			Negotiated string
			Error      any
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.IDs {
		if got := ValidJSONRPCID(c.Input); got != c.Valid {
			t.Errorf("ID %s: got %v want %v", c.Input, got, c.Valid)
		}
	}
	for _, c := range fixture.Responses {
		if got := ValidJSONRPCResponse(c.Input); got != c.Valid {
			t.Errorf("response %s: got %v want %v", c.Input, got, c.Valid)
		}
	}
	for _, c := range fixture.Versions {
		if got := ExactOrFallback(c.Accepted, c.Preferred); got != c.Negotiated {
			t.Error(got, c.Negotiated)
		}
		b, _ := json.Marshal(ProtocolVersionError(c.Accepted))
		var got any
		_ = json.Unmarshal(b, &got)
		if !reflect.DeepEqual(got, c.Error) {
			t.Errorf("got %#v want %#v", got, c.Error)
		}
	}
	versions := ProtocolVersions()
	versions[0] = "mutated"
	if ProtocolVersions()[0] != "2025-03-26" {
		t.Fatal("mutable global protocol list")
	}
}

func TestMalformedJSON(t *testing.T) {
	for _, raw := range []string{"", "[", "{} {}", "null true", "01", "NaN", "{\"id\":1,", "{\"id\":1} trailing"} {
		if ValidJSONRPCID([]byte(raw)) || ValidJSONRPCResponse([]byte(raw)) {
			t.Errorf("accepted %q", raw)
		}
	}
}

func FuzzJSONRPCValidation(f *testing.F) {
	for _, s := range []string{"null", "1", "true", "1.0", "123456789012345678901234567890", `{\"id\":1,\"result\":null}`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) { _ = ValidJSONRPCID([]byte(raw)); _ = ValidJSONRPCResponse([]byte(raw)) })
}
