package repository

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ConceptFrontmatter is the validated metadata model. APIs return independent
// slices. Normally revalidate modified values; service model-copy updates use
// SerializeCopiedConcept to retain the source's deliberate validation bypass.
type ConceptFrontmatter struct {
	// The reference's unvalidated enum default cannot be emitted by ruamel.
	// Keep its provenance for serialisation parity; explicit active is valid.
	defaultStatus bool
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	Description   *string   `json:"description"`
	Aliases       []string  `json:"aliases"`
	Tags          []string  `json:"tags"`
	SourceRefs    []string  `json:"source_refs"`
	Supersedes    []string  `json:"supersedes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	UpdatedBy     string    `json:"updated_by"`
}

// SetCopiedStatus clears enum-default provenance when a patch explicitly
// supplies a validated status string through model_copy.
func (m *ConceptFrontmatter) SetCopiedStatus(status string) {
	m.Status = status
	m.defaultStatus = false
}

type ConceptDocument struct {
	Frontmatter ConceptFrontmatter `json:"frontmatter"`
	Body        string             `json:"body"`
}
type FrontmatterError struct{ Message string }

func (e *FrontmatterError) Error() string { return e.Message }
func validationError() error              { return &FrontmatterError{Message: "frontmatter validation failed"} }
func ControlledTypes() []string {
	return []string{"concept", "instance", "person", "project", "service", "system"}
}
func textControl(value string) bool {
	for _, r := range value {
		if r < 32 || r == 127 {
			return true
		}
	}
	return false
}
func conceptSpace(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }

// NormalizeConceptBody retains leading indentation, strips trailing Python
// whitespace on every line, folds newlines, and removes only outer blank lines.
func NormalizeConceptBody(body string) string {
	body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(body, "\n")
	for i := range lines {
		lines[i] = strings.TrimRightFunc(lines[i], conceptSpace)
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

// ValidateConceptMetadata accepts decoded JSON/YAML values, preserving the
// reference's deliberately non-strict schema_version coercion. It does not
// silently stringify numbers, strip titles, restrict IDs to UUIDs or constrain
// descriptions/list strings beyond the Python model's actual rules.
func ValidateConceptMetadata(value any) (ConceptFrontmatter, error) {
	model := ConceptFrontmatter{defaultStatus: true, SchemaVersion: 1, Status: "active", Aliases: []string{}, Tags: []string{}, SourceRefs: []string{}, Supersedes: []string{}}
	invalid := func() (ConceptFrontmatter, error) { return ConceptFrontmatter{}, validationError() }
	metadata, ok := value.(map[string]any)
	if !ok {
		return invalid()
	}
	for key := range metadata {
		switch key {
		case "schema_version", "id", "type", "title", "status", "description", "aliases", "tags", "source_refs", "supersedes", "created_at", "updated_at", "updated_by":
		default:
			return invalid()
		}
	}
	if v, exists := metadata["schema_version"]; exists && !schemaOne(v) {
		return invalid()
	}
	for key, target := range map[string]*string{"id": &model.ID, "type": &model.Type, "title": &model.Title, "updated_by": &model.UpdatedBy} {
		v, ok := metadata[key].(string)
		if !ok || v == "" {
			return invalid()
		}
		*target = v
	}
	if textControl(model.Title) || textControl(model.UpdatedBy) {
		return invalid()
	}
	found := false
	for _, kind := range ControlledTypes() {
		if model.Type == kind {
			found = true
		}
	}
	if !found {
		return invalid()
	}
	if v, exists := metadata["status"]; exists {
		status, ok := v.(string)
		if !ok || (status != "active" && status != "deprecated" && status != "tombstone") {
			return invalid()
		}
		model.Status = status
		model.defaultStatus = false
	}
	if v := metadata["description"]; v != nil {
		description, ok := v.(string)
		if !ok {
			return invalid()
		}
		model.Description = &description
	}
	for key, target := range map[string]*[]string{"aliases": &model.Aliases, "tags": &model.Tags, "source_refs": &model.SourceRefs, "supersedes": &model.Supersedes} {
		if v, exists := metadata[key]; exists {
			items, ok := stringTuple(v)
			if !ok {
				return invalid()
			}
			*target = items
		}
	}
	for key, target := range map[string]*time.Time{"created_at": &model.CreatedAt, "updated_at": &model.UpdatedAt} {
		v, ok := conceptTimestamp(metadata[key])
		if !ok {
			return invalid()
		}
		*target = v
	}
	return model, nil
}

var schemaInteger = regexp.MustCompile(`^[+-]?[0-9](?:_?[0-9])*(?:\.0+)?$`)

func schemaOne(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v == 1
	case int64:
		return v == 1
	case float64:
		return v == 1
	case json.Number:
		return v == "1" || numericOne(string(v))
	case string:
		v = strings.TrimSpace(v)
		if !schemaInteger.MatchString(v) {
			return false
		}
		return numericOne(strings.ReplaceAll(v, "_", ""))
	default:
		return false
	}
}
func numericOne(value string) bool {
	number, err := strconv.ParseFloat(value, 64)
	return err == nil && number == 1
}
func stringTuple(value any) ([]string, bool) {
	items := []string{}
	switch v := value.(type) {
	case []string:
		items = append(items, v...)
	case []any:
		for _, item := range v {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			items = append(items, text)
		}
	default:
		return nil, false
	}
	sort.Strings(items)
	out := []string{}
	for _, item := range items {
		if item != "" && (len(out) == 0 || out[len(out)-1] != item) {
			out = append(out, item)
		}
	}
	return out, true
}

var timestampNumber = regexp.MustCompile(`^[+-]?(?:[0-9]+|(?:[0-9]+\.[0-9]*|\.[0-9]+)(?:[eE][+-]?[0-9]+)?)$`)
var timestampISO = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2})[Tt ]([0-9]{2}:[0-9]{2})(?::([0-9]{2})([.,][0-9]+)?)?([Zz]|[+-][0-9]{2}:?[0-9]{2})$`)

func conceptTimestamp(value any) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		// YAML timestamps carry explicit UTC/offset zones. The YAML reader must
		// keep naive dates as text so this case never invents timezone information.
		t := v.UTC()
		return t, t.Year() >= 1 && t.Year() <= 9999
	case string:
		if timestampNumber.MatchString(v) {
			n, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return time.Time{}, false
			}
			return epochTime(n)
		}
		parts := timestampISO.FindStringSubmatch(v)
		if parts == nil {
			return time.Time{}, false
		}
		sec := parts[3]
		if sec == "" {
			sec = "00"
		}
		fraction := parts[4]
		if len(fraction) > 7 {
			fraction = fraction[:7]
		}
		fraction = strings.ReplaceAll(fraction, ",", ".")
		zone := strings.ToUpper(parts[5])
		if len(zone) == 5 {
			zone = zone[:3] + ":" + zone[3:]
		}
		if zone != "Z" {
			hours, _ := strconv.Atoi(zone[1:3])
			minutes, _ := strconv.Atoi(zone[4:])
			if hours > 23 || minutes > 59 {
				return time.Time{}, false
			}
		}
		t, err := time.Parse(time.RFC3339, parts[1]+"T"+parts[2]+":"+sec+fraction+zone)
		if err != nil {
			return time.Time{}, false
		}
		t = t.UTC()
		return t, t.Year() >= 1 && t.Year() <= 9999
	case json.Number:
		n, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return time.Time{}, false
		}
		if strings.ContainsAny(string(v), ".eE") {
			return epochFloat(n)
		}
		return epochTime(n)
	case int:
		return epochTime(float64(v))
	case int64:
		return epochTime(float64(v))
	case float64:
		return epochFloat(v)
	default:
		return time.Time{}, false
	}
}

// Pydantic's Python-float path floors the integer part and then adds the
// absolute fractional part. Numeric strings use the conventional signed epoch.
// Preserve the source quirk, including its independent millisecond cutoffs.
func epochFloat(value float64) (time.Time, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return time.Time{}, false
	}
	whole := math.Floor(value)
	fraction := math.Abs(value - math.Trunc(value))
	if math.Abs(whole) > 20000000000 {
		whole /= 1000
	}
	if math.Abs(value) > 20000000000 {
		fraction /= 1000
	}
	if whole < -62135596801 || whole > 253402300800 {
		return time.Time{}, false
	}
	seconds, remainder := math.Modf(whole)
	micro := int64(math.Round((remainder + fraction) * 1e6))
	t := time.Unix(int64(seconds), micro*1000).UTC()
	return t, t.Year() >= 1 && t.Year() <= 9999
}
func epochTime(value float64) (time.Time, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return time.Time{}, false
	}
	if math.Abs(value) > 20000000000 {
		value /= 1000
	}
	if value < -62135596800 || value >= 253402300800 {
		return time.Time{}, false
	}
	sec, fraction := math.Modf(value)
	micro := int64(math.Round(fraction * 1e6))
	t := time.Unix(int64(sec), micro*1000).UTC()
	return t, t.Year() >= 1 && t.Year() <= 9999
}

// NewConceptFrontmatter creates a UTC, second-resolution timestamp and UUIDv4.
// Creation remains validated through the same schema as parsed documents.
func NewConceptFrontmatter(title, conceptType, updatedBy string) (ConceptFrontmatter, error) {
	return newConceptFrontmatter(title, conceptType, updatedBy, time.Now, rand.Reader)
}
func newConceptFrontmatter(title, conceptType, updatedBy string, now func() time.Time, random io.Reader) (ConceptFrontmatter, error) {
	var id [16]byte
	if _, err := io.ReadFull(random, id[:]); err != nil {
		return ConceptFrontmatter{}, err
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	stamp := now().UTC().Truncate(time.Second)
	return ValidateConceptMetadata(map[string]any{"id": fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), "type": conceptType, "title": title, "created_at": stamp, "updated_at": stamp, "updated_by": updatedBy})
}
