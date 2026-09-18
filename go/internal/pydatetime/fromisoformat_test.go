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
