package umcp

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestPythonHTTPRules(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/umcp-http-rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Accepts []struct {
			Input string
			JSON  bool
			SSE   bool
		}
		ContentTypes []struct {
			Input string
			Valid bool
		} `json:"content_types"`
		Counts []struct {
			Counts  map[string]int
			Version string
			Invalid bool
		}
		Headers []struct {
			Headers   map[string]string
			Ambiguous bool
		}
		Statuses []struct {
			Code int
			Line string
		}
		Responses []struct {
			Status      int
			Body        string
			ContentType *string `json:"content_type"`
			Headers     [][2]string
			MaxBytes    int `json:"max_bytes"`
			Valid       bool
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Accepts {
		if MediaAcceptsJSON(c.Input) != c.JSON || MediaAcceptsEventStream(c.Input) != c.SSE {
			t.Errorf("accept %q", c.Input)
		}
	}
	for _, c := range fixture.ContentTypes {
		if ContentTypeIsJSON(c.Input) != c.Valid {
			t.Errorf("content type %q", c.Input)
		}
	}
	for _, c := range fixture.Counts {
		if HasSingletonHeaderViolations(c.Counts, c.Version) != c.Invalid {
			t.Error(c)
		}
	}
	for _, c := range fixture.Headers {
		if HasAmbiguousSingletonValues(c.Headers) != c.Ambiguous {
			t.Error(c)
		}
	}
	for _, c := range fixture.Statuses {
		if got := HTTPStatusLine(c.Code); got != c.Line {
			t.Errorf("status %d: %q != %q", c.Code, got, c.Line)
		}
	}
	for _, c := range fixture.Responses {
		r := &HTTPResponse{Status: c.Status, Body: []byte(c.Body), ContentType: c.ContentType, Headers: c.Headers}
		if ValidateHTTPResponse(r, c.MaxBytes) != c.Valid {
			t.Error(c)
		}
	}
	if ValidateHTTPResponse(nil, 100) {
		t.Fatal("nil response accepted")
	}
	names := SingletonHTTPHeaders()
	names[0] = "modified"
	if SingletonHTTPHeaders()[0] != "host" {
		t.Fatal("mutable global")
	}
}

func TestPythonQuality(t *testing.T) {
	for _, c := range []struct {
		In   string
		Want float64
	}{{"  +.5  ", .5}, {"1_0", 10}, {"0x1p0", 0}, {"inf", math.Inf(1)}, {"-infinity", math.Inf(-1)}, {"1e999", math.Inf(1)}, {"1e-999", 0}, {"1e-99999999999999999999999999999", 0}, {"١.٥", 1.5}, {"𝟙.𝟝", 1.5}, {"", 0}, {"1__0", 0}, {"wat", 0}} {
		if got := pythonQuality(c.In); got != c.Want {
			t.Errorf("quality %q %v != %v", c.In, got, c.Want)
		}
	}
	for _, s := range []string{"nan", "+nan", "-nan"} {
		if !math.IsNaN(pythonQuality(s)) {
			t.Error(s)
		}
	}
	if !MediaAccepts("image/*", "IMAGE/PNG", "application/json") || MediaAccepts("image/*", "text/plain") {
		t.Fatal("generic media matching")
	}
}

func FuzzHTTPRules(f *testing.F) {
	for _, s := range []string{"application/json", "*/*;q=0", "text/event-stream;q=nan", "application/json;q=١_٠"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_ = MediaAcceptsJSON(s)
		_ = MediaAcceptsEventStream(s)
		_ = ContentTypeIsJSON(s)
		_ = HasAmbiguousSingletonValues(map[string]string{"origin": s})
		_ = ValidateHTTPResponse(&HTTPResponse{Status: 200, ContentType: &s, Headers: [][2]string{{s, s}}}, 100)
	})
}
