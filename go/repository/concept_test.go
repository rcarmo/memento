package repository

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestConceptSchemaReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/concept-schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Metadata []struct {
			Input    json.RawMessage
			Expected *ConceptFrontmatter
		}
		Bodies []struct{ Input, Expected string }
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for i, c := range fixtures.Metadata {
		var input any
		decoder := json.NewDecoder(bytes.NewReader(c.Input))
		decoder.UseNumber()
		if err = decoder.Decode(&input); err != nil {
			t.Fatal(err)
		}
		actual, err := ValidateConceptMetadata(input)
		if c.Expected == nil {
			if err == nil {
				t.Errorf("case %d unexpectedly valid: %s", i, c.Input)
			}
			continue
		}
		if err != nil {
			t.Errorf("case %d unexpectedly invalid: %s", i, c.Input)
			continue
		}
		// time.Time location internals differ on unmarshal; compare UTC instants.
		if !actual.CreatedAt.Equal(c.Expected.CreatedAt) || !actual.UpdatedAt.Equal(c.Expected.UpdatedAt) {
			t.Errorf("case %d timestamps %s,%s != %s,%s input %s", i, actual.CreatedAt, actual.UpdatedAt, c.Expected.CreatedAt, c.Expected.UpdatedAt, c.Input)
		}
		actual.defaultStatus = false
		actual.CreatedAt = c.Expected.CreatedAt
		actual.UpdatedAt = c.Expected.UpdatedAt
		if !reflect.DeepEqual(actual, *c.Expected) {
			t.Errorf("case %d: %+v != %+v", i, actual, *c.Expected)
		}
	}
	for _, c := range fixtures.Bodies {
		if got := NormalizeConceptBody(c.Input); got != c.Expected {
			t.Fatal(c, got)
		}
	}
}
func TestConceptTypedAndCreation(t *testing.T) {
	stamp := time.Date(2026, 9, 18, 12, 0, 0, 123, time.FixedZone("test", 3600))
	model, err := newConceptFrontmatter("Title", "concept", "agent", func() time.Time { return stamp }, bytes.NewReader(make([]byte, 16)))
	if err != nil {
		t.Fatal(err)
	}
	if model.ID != "00000000-0000-4000-8000-000000000000" || model.CreatedAt.Nanosecond() != 0 || model.CreatedAt.Hour() != 11 {
		t.Fatal(model)
	}
	actual, err := NewConceptFrontmatter("Title", "project", "agent")
	if err != nil || actual.ID[14] != '4' {
		t.Fatal(actual, err)
	}
	if _, err = newConceptFrontmatter("x", "concept", "y", time.Now, strings.NewReader("")); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	if _, err = NewConceptFrontmatter("", "project", "agent"); err == nil {
		t.Fatal("invalid title")
	}
	var failure *FrontmatterError
	if !errors.As(err, &failure) || err.Error() != "frontmatter validation failed" {
		t.Fatal(err)
	}
	for _, c := range []struct {
		Value any
		Want  bool
	}{{int(1), true}, {int64(1), true}, {float64(1), true}, {float64(1.1), false}, {"1e0", false}, {json.Number("1e0"), true}, {[]int{1}, false}} {
		if got := schemaOne(c.Value); got != c.Want {
			t.Fatal(c, got)
		}
	}
	values := []string{"z", "a", "a"}
	got, ok := stringTuple(values)
	if !ok || strings.Join(got, ",") != "a,z" || values[0] != "z" {
		t.Fatal(got, values)
	}
	for _, value := range []any{stamp, int(1), int64(1), float64(1), json.Number("1"), json.Number("invalid"), json.Number("1e999"), strings.Repeat("9", 1000), true, math.NaN(), math.Inf(1), math.Inf(-1), time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)} {
		_, _ = conceptTimestamp(value)
	}
}
func FuzzConceptMetadata(f *testing.F) {
	for _, value := range []string{"{}", "null", `{"id":"x","title":"x","type":"concept"}`} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		var v any
		decoder := json.NewDecoder(strings.NewReader(value))
		decoder.UseNumber()
		if decoder.Decode(&v) == nil {
			_, _ = ValidateConceptMetadata(v)
		}
		_ = NormalizeConceptBody(value)
	})
}

func TestEpochRejectedRanges(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 1e20, -1e20} {
		if _, ok := epochTime(value); ok {
			t.Fatal(value)
		}
		if _, ok := epochFloat(value); ok {
			t.Fatal(value)
		}
	}
}

func TestConceptParseReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/concept-frontmatter.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Parse []struct {
			Input, Error string
			Expected     *ConceptDocument
		}
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for i, c := range fixtures.Parse {
		actual, err := ParseConceptText(c.Input)
		if c.Error != "" {
			if errorText(err) != c.Error {
				t.Errorf("case %d: %v != %s", i, err, c.Error)
			}
			continue
		}
		if err != nil {
			t.Errorf("case %d: %v input %s", i, err, c.Input)
			continue
		}
		if !actual.Frontmatter.CreatedAt.Equal(c.Expected.Frontmatter.CreatedAt) || !actual.Frontmatter.UpdatedAt.Equal(c.Expected.Frontmatter.UpdatedAt) {
			t.Fatal(i, actual, c.Expected)
		}
		actual.Frontmatter.CreatedAt = c.Expected.Frontmatter.CreatedAt
		actual.Frontmatter.UpdatedAt = c.Expected.Frontmatter.UpdatedAt
		if !reflect.DeepEqual(actual, *c.Expected) {
			t.Errorf("case %d %+v != %+v", i, actual, *c.Expected)
		}
	}
}

func TestConceptSerializeReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/concept-frontmatter.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Serialize []struct {
			Metadata              json.RawMessage
			Body, Expected, Error string
		}
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for i, c := range fixtures.Serialize {
		var metadata any
		decoder := json.NewDecoder(bytes.NewReader(c.Metadata))
		decoder.UseNumber()
		_ = decoder.Decode(&metadata)
		model, err := ValidateConceptMetadata(metadata)
		if c.Error == "validation" {
			if err == nil {
				t.Errorf("case %d validation accepted", i)
			}
			continue
		}
		if err != nil {
			t.Fatal(i, err)
		}
		text, err := SerializeConcept(ConceptDocument{Frontmatter: model, Body: c.Body})
		if c.Error == "serialization" {
			if err == nil {
				t.Errorf("case %d serialization accepted", i)
			}
			continue
		}
		if err != nil || text != c.Expected {
			t.Errorf("case %d: %v\n%q\n!=\n%q", i, err, text, c.Expected)
		}
	}
}

func TestSerializeRejectsModifiedMetadata(t *testing.T) {
	if _, err := SerializeConcept(ConceptDocument{}); err == nil {
		t.Fatal("unvalidated metadata emitted")
	}
}
