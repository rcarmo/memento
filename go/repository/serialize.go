package repository

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// SerializeConcept emits the source metadata order, scalar quoting and UTC
// second-resolution timestamps. Default enum status preserves its source error.
func SerializeConcept(document ConceptDocument) (string, error) {
	m := document.Frontmatter
	if m.defaultStatus {
		return "", &FrontmatterError{Message: "cannot represent an object: <ConceptStatus.ACTIVE: 'active'>"}
	}
	// Go structs can be changed after construction; reapply the frozen Python
	// model's constraints before emitting repository content.
	metadata := map[string]any{"schema_version": m.SchemaVersion, "id": m.ID, "type": m.Type, "title": m.Title, "status": m.Status, "aliases": m.Aliases, "tags": m.Tags, "source_refs": m.SourceRefs, "supersedes": m.Supersedes, "created_at": m.CreatedAt, "updated_at": m.UpdatedAt, "updated_by": m.UpdatedBy}
	if m.Description != nil {
		metadata["description"] = *m.Description
	}
	validated, err := ValidateConceptMetadata(metadata)
	if err != nil {
		return "", err
	}
	m = validated
	var out strings.Builder
	out.WriteString("---\nschema_version: 1\n")
	scalar := func(key, value string) {
		out.WriteString(key + ": " + wrapYAMLScalar(yamlString(value, 2), len(key)+2, 2) + "\n")
	}
	scalar("id", m.ID)
	scalar("type", m.Type)
	scalar("title", m.Title)
	scalar("status", m.Status)
	if m.Description != nil {
		scalar("description", *m.Description)
	}
	for _, field := range []struct {
		Key    string
		Values []string
	}{{"aliases", m.Aliases}, {"tags", m.Tags}, {"source_refs", m.SourceRefs}, {"supersedes", m.Supersedes}} {
		if len(field.Values) == 0 {
			out.WriteString(field.Key + ": []\n")
			continue
		}
		out.WriteString(field.Key + ":\n")
		for _, value := range field.Values {
			out.WriteString("  - " + wrapYAMLScalar(yamlString(value, 4), 4, 4) + "\n")
		}
	}
	scalar("created_at", m.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"))
	scalar("updated_at", m.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"))
	scalar("updated_by", m.UpdatedBy)
	out.WriteString("---\n" + NormalizeConceptBody(document.Body) + "\n")
	return out.String(), nil
}

func wrapYAMLScalar(value string, column, indent int) string {
	// ruamel width=4096: plain words move as a unit; quoted strings wrap
	// only on isolated interior spaces, retaining quote style and indentation.
	if strings.HasPrefix(value, "\"") {
		return value
	}
	quoted := strings.HasPrefix(value, "'")
	var out strings.Builder
	for start := 0; start < len(value); {
		space := value[start] == ' '
		end := start + 1
		for end < len(value) && (value[end] == ' ') == space {
			end++
		}
		chunk := value[start:end]
		width := utf8.RuneCountInString(chunk)
		wrap := false
		if space {
			wrap = width == 1 && start > 0 && end < len(value) && ((quoted && column > 4096) || (!quoted && column >= 4096))
		} else if !quoted {
			wrap = column+width > 4096 && column > indent
		}
		if wrap {
			out.WriteByte('\n')
			out.WriteString(strings.Repeat(" ", indent))
			column = indent
		}
		if !wrap || !space {
			out.WriteString(chunk)
			column += width
		}
		start = end
	}
	return out.String()
}

var yaml12Number = regexp.MustCompile(`^[+-]?(?:[0-9][0-9_]*|0b[01_]+|0o?[0-7_]+|0x[0-9a-fA-F_]+|[0-9][0-9_]*\.[0-9_]*(?:[eE][+-]?[0-9]+)?|[0-9][0-9_]*[eE][+-]?[0-9]+|\.[0-9_]+(?:[eE][+-][0-9]+)?)$`)

func yamlString(value string, indent int) string {
	if value == "" {
		return "''"
	}
	chars := []rune(value)
	plain := true
	double := false
	whitespace := func(r rune) bool { return strings.ContainsRune("\x00 \t\r\n\u0085\u2028\u2029", r) }
	if strings.HasPrefix(value, "---") || strings.HasPrefix(value, "...") {
		plain = false
	}
	for i, r := range chars {
		before := i == 0 || whitespace(chars[i-1])
		after := i == len(chars)-1 || whitespace(chars[i+1])
		if i == 0 && (strings.ContainsRune("#,[]{}&*!|>'\"%@`", r) || (r == '?' || r == ':' || r == '-') && after) {
			plain = false
		}
		if i > 0 && (r == ':' && after || r == '#' && before) {
			plain = false
		}
		if (i == 0 || i == len(chars)-1) && whitespace(r) {
			plain = false
		}
		if strings.ContainsRune("\n\u0085\u2028\u2029", r) {
			plain = false
			if i > 0 && chars[i-1] == ' ' || i+1 < len(chars) && chars[i+1] == ' ' {
				double = true
			}
		}
		if r == '\n' || r == '\r' || r == '\t' || r < 32 || r == 127 || r >= 128 && r < 160 && r != 0x85 || r == 0xfeff || r == 0xfffe || r == 0xffff {
			double = true
			plain = false
		}
	}
	var probe yaml.Node
	if yaml.Unmarshal([]byte(value), &probe) == nil && len(probe.Content) > 0 && probe.Content[0].Kind == yaml.ScalarNode && probe.Content[0].Tag != "!!str" {
		plain = false
	}
	if yaml12Number.MatchString(value) || value == "<<" || value == "=" {
		plain = false
	}
	// yaml.v3 deliberately quotes YAML 1.1 booleans; ruamel's default 1.2 does not.
	if plain {
		return value
	}
	if strings.ContainsRune(value, '\'') {
		double = true
	}
	if double {
		return yamlDoubleQuoted(value)
	}
	// Non-LF Unicode breaks remain literal in ruamel single quoted strings.
	var quoted strings.Builder
	quoted.WriteByte('\'')
	for i, r := range chars {
		quoted.WriteRune(r)
		if strings.ContainsRune("\u0085\u2028\u2029", r) && (i+1 == len(chars) || !strings.ContainsRune("\u0085\u2028\u2029", chars[i+1])) {
			quoted.WriteString(strings.Repeat(" ", indent))
		}
	}
	quoted.WriteByte('\'')
	return quoted.String()
}
func yamlDoubleQuoted(value string) string {
	escapes := map[rune]string{0: "0", 7: "a", 8: "b", 9: "t", 10: "n", 11: "v", 12: "f", 13: "r", 27: "e", 34: "\"", 92: "\\", 0x85: "N", 0x2028: "L", 0x2029: "P"}
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		if escape, ok := escapes[r]; ok {
			out.WriteByte('\\')
			out.WriteString(escape)
		} else if r < 32 || r >= 127 && r < 160 {
			fmt.Fprintf(&out, "\\x%02X", r)
		} else if r == 0xfeff || r == 0xfffe || r == 0xffff {
			fmt.Fprintf(&out, "\\u%04X", r)
		} else {
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
	return out.String()
}
