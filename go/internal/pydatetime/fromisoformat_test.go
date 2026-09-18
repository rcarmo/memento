package pydatetime

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

func TestManifestTimestampCorpus(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/manifest-timestamps.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Value           any
		Expected, Error string
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		got, err := ParseAware(tc.Value)
		if tc.Error != "" {
			want := ErrInvalid
			if tc.Error == "manifest timestamps must be timezone-aware" {
				want = ErrNaive
			} else if tc.Error == "date value out of range" {
				want = ErrOverflow
			}
			if !errors.Is(err, want) {
				t.Fatal(tc.Value, err, want)
			}
			continue
		}
		text := got.UTC().Format("2006-01-02T15:04:05")
		if got.Nanosecond()/1000 != 0 {
			text += got.UTC().Format(".000000")
		}
		text += "Z"
		if err != nil || text != tc.Expected {
			t.Fatal(tc.Value, got, err, tc.Expected)
		}
	}
	if _, err := ParseAware(struct{}{}); !errors.Is(err, ErrNaive) {
		t.Fatal(err)
	}
	now := time.Now()
	got, err := ParseAware(now)
	if err != nil || !got.Equal(now.UTC().Truncate(time.Microsecond)) {
		t.Fatal(got, err)
	}
	if _, err = ParseAware("0001-01-01T00:00+01"); !errors.Is(err, ErrOverflow) {
		t.Fatal(err)
	}
}
func TestParseErrorReason(t *testing.T) {
	err := parseError(ErrInvalid, "specific")
	if !errors.Is(err, ErrInvalid) || Reason(err) != "specific" || Reason(ErrInvalid) != "" || err.Error() != "invalid ISO 8601 datetime: specific" {
		t.Fatal(err, Reason(err))
	}
}
func TestParseJSON(t *testing.T) {
	cases := []struct {
		value  any
		strict bool
		want   string
	}{{json.Number("0"), false, "1970-01-01T00:00:00Z"}, {json.Number("-1"), false, "1969-12-31T23:59:59Z"}, {json.Number("1.0000005"), false, "1970-01-01T00:00:01.000001Z"}, {json.Number("20000000000"), false, "2603-10-11T11:33:20Z"}, {json.Number("20000000001"), false, "1970-08-20T11:33:20.001Z"}, {"20000000000.5", true, "1970-08-20T11:33:20.0005Z"}}
	for _, tc := range cases {
		got, err := ParseJSON(tc.value, tc.strict)
		if err != nil || got.Format(time.RFC3339Nano) != tc.want {
			t.Fatal(tc, got, err)
		}
	}
	for _, accepted := range []string{"2026-01-01t12:34z", "2026-01-01_12:34:56,123+01:30"} {
		if _, err := ParseJSON(accepted, true); err != nil {
			t.Fatal(accepted, err)
		}
	}
	for _, rejected := range []string{"20260101T123456+0130", "2026-W01-1T00:00:00Z", "2026-01-01🍀12:34:56Z", "2026-01-01T12Z", "2026-01-01T00:00:00+00:00:30"} {
		if _, err := ParseJSON(rejected, true); !errors.Is(err, ErrInvalid) {
			t.Fatal(rejected, err)
		}
	}
	for _, tc := range []struct {
		value  any
		strict bool
	}{{json.Number("0"), true}, {true, false}, {json.Number("bad"), false}, {json.Number("1e9999"), false}, {"bad", false}, {json.Number("999999999999999999999999999999"), false}} {
		if _, err := ParseJSON(tc.value, tc.strict); err == nil {
			t.Fatal(tc)
		}
	}
}
func FuzzParseAware(f *testing.F) {
	for _, text := range []string{"", "2026-01-01T00:00:00Z", "2026W011🍀12:34:56.1234567+0130", "0001-01-01T00:00+01", "bad"} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 2048 {
			return
		}
		got, err := ParseAware(text)
		if err != nil {
			return
		}
		if got.Year() < 1 || got.Year() > 9999 || got.Nanosecond()%1000 != 0 {
			t.Fatal(got)
		}
		again, err := ParseAware(got.Format("2006-01-02T15:04:05.999999Z07:00"))
		if err != nil || !again.Equal(got) {
			t.Fatal(got, again, err)
		}
	})
}
