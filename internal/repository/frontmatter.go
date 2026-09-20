package repository

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

func ParseConceptText(text string) (ConceptDocument, error) {
	invalid := func(message string) (ConceptDocument, error) {
		return ConceptDocument{}, &FrontmatterError{Message: message}
	}
	if !utf8.ValidString(text) {
		return invalid("invalid frontmatter")
	}
	text = strings.TrimFunc(text, conceptSpace)
	metadata := any(map[string]any{})
	body := text
	format, raw, content, ok := splitConceptText(text)
	if ok {
		var err error
		if format == "json" {
			decoder := json.NewDecoder(strings.NewReader(raw))
			decoder.UseNumber()
			err = decoder.Decode(&metadata)
			if err == nil {
				var extra any
				if decoder.Decode(&extra) != io.EOF {
					err = fmt.Errorf("trailing JSON")
				}
			}
		} else {
			metadata, err = parseYAMLMetadata(raw)
		}
		if err != nil {
			return invalid("invalid frontmatter")
		}
		if _, ok := metadata.(map[string]any); !ok {
			metadata = map[string]any{}
		}
		body = strings.TrimFunc(content, conceptSpace)
	}
	model, err := ValidateConceptMetadata(metadata)
	if err != nil {
		return ConceptDocument{}, err
	}
	return ConceptDocument{Frontmatter: model, Body: NormalizeConceptBody(body)}, nil
}
func ParseConceptFile(path string) (ConceptDocument, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ConceptDocument{}, err
	}
	// Python text-file reads apply universal newline translation before detection.
	text := strings.ReplaceAll(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\r", "\n")
	return ParseConceptText(text)
}

func splitConceptText(text string) (format, metadata, body string, ok bool) {
	if strings.HasPrefix(text, "{\n") || text == "{" {
		// JSONHandler's boundary is a whole line consisting of either brace.
		offsets := lineOffsets(text)
		for _, pos := range offsets[1:] {
			end := strings.IndexByte(text[pos:], '\n')
			if end < 0 {
				end = len(text) - pos
			}
			line := text[pos : pos+end]
			if line == "{" || line == "}" {
				return "json", "{" + text[1:pos] + "}", text[pos+end:], true
			}
		}
		return "", "", "", false
	}
	boundaries := []int{}
	ends := []int{}
	for _, pos := range lineOffsets(text) {
		i := pos
		for i < len(text) && text[i] == '-' {
			i++
		}
		if i-pos < 3 {
			continue
		}
		// Python \s* includes newlines, greedily consuming blank lines to the
		// last end-of-line it can reach, without consuming content indentation.
		last := -1
		for {
			if i == len(text) {
				last = i
				break
			}
			if text[i] == '\n' {
				last = i
			}
			r, size := utf8.DecodeRuneInString(text[i:])
			if !conceptSpace(r) {
				break
			}
			i += size
		}
		if last >= 0 {
			boundaries = append(boundaries, pos)
			ends = append(ends, last)
		}
	}
	if len(boundaries) < 2 || boundaries[0] != 0 {
		return "", "", "", false
	}
	return "yaml", text[ends[0]:boundaries[1]], text[ends[1]:], true
}
func lineOffsets(text string) []int {
	out := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' && i+1 < len(text) {
			out = append(out, i+1)
		}
	}
	return out
}

func parseYAMLMetadata(text string) (any, error) {
	var node yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(text))
	if err := decoder.Decode(&node); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("multiple YAML documents")
	}
	return yamlValue(node.Content[0], map[*yaml.Node]bool{}, 0)
}

type recursiveYAML struct{}

func yamlValue(node *yaml.Node, active map[*yaml.Node]bool, depth int) (any, error) {
	remaining := 100000
	return yamlValueBounded(node, active, depth, &remaining)
}
func yamlValueBounded(node *yaml.Node, active map[*yaml.Node]bool, depth int, remaining *int) (any, error) {
	*remaining--
	if *remaining < 0 {
		return nil, fmt.Errorf("YAML expansion limit")
	}
	// Limits fail closed at the parser boundary; they are not a claim of Python
	// resource-limit parity. Recursive graphs cannot satisfy the metadata schema.
	if active[node] {
		return recursiveYAML{}, nil
	}
	if depth > 100 {
		return nil, fmt.Errorf("YAML depth limit")
	}
	if node.Kind == yaml.MappingNode && node.Tag == "!!set" {
		out := []any{}
		for i := 0; i < len(node.Content); i += 2 {
			item, err := yamlValueBounded(node.Content[i], active, depth+1, remaining)
			if err != nil {
				return nil, err
			}
			out = append(out, item)
		}
		return out, nil
	}
	if node.Kind == yaml.MappingNode && node.Tag != "!!map" || node.Kind == yaml.SequenceNode && node.Tag != "!!seq" {
		return nil, fmt.Errorf("unsupported YAML collection tag")
	}
	active[node] = true
	defer delete(active, node)
	switch node.Kind {
	case yaml.AliasNode:
		return yamlValueBounded(node.Alias, active, depth+1, remaining)
	case yaml.MappingNode:
		out := map[string]any{}
		// YAML merge sources precede explicit pairs; duplicate explicit keys are
		// last-wins in PyYAML, unlike the default yaml.v3 struct decoder.
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Tag == "!!merge" {
				merge := *node.Content[i+1]
				// PyYAML flattens merge collections before tag construction.
				if merge.Kind == yaml.SequenceNode {
					merge.Tag = "!!seq"
				}
				if merge.Kind == yaml.MappingNode {
					merge.Tag = "!!map"
				}
				value, err := yamlValueBounded(&merge, active, depth+1, remaining)
				if err != nil {
					return nil, err
				}
				sources := []any{value}
				if items, ok := value.([]any); ok {
					sources = items
				}
				for j := len(sources) - 1; j >= 0; j-- {
					m, ok := sources[j].(map[string]any)
					if !ok {
						return nil, fmt.Errorf("invalid YAML merge")
					}
					for k, v := range m {
						out[k] = v
					}
				}
			}
		}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Tag == "!!merge" {
				continue
			}
			name, err := yamlValueBounded(key, active, depth+1, remaining)
			if err != nil {
				return nil, err
			}
			k, ok := name.(string)
			if !ok {
				return nil, fmt.Errorf("non-string metadata key")
			}
			value, err := yamlValueBounded(node.Content[i+1], active, depth+1, remaining)
			if err != nil {
				return nil, err
			}
			out[k] = value
		}
		return out, nil
	case yaml.SequenceNode:
		out := []any{}
		for _, child := range node.Content {
			value, err := yamlValueBounded(child, active, depth+1, remaining)
			if err != nil {
				return nil, err
			}
			out = append(out, value)
		}
		return out, nil
	case yaml.ScalarNode:
		return yamlScalar(node)
	default:
		return nil, fmt.Errorf("unsupported YAML node")
	}
}

var pyYAMLFloat = regexp.MustCompile(`^(?:[-+]?[0-9][0-9_]*\.[0-9_]*(?:[eE][-+][0-9]+)?|\.[0-9][0-9_]*(?:[eE][-+][0-9]+)?|[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN))$`)
var yamlBool = regexp.MustCompile(`^(?:yes|Yes|YES|no|No|NO|true|True|TRUE|false|False|FALSE|on|On|ON|off|Off|OFF)$`)
var yamlSexagesimal = regexp.MustCompile(`^[-+]?[1-9][0-9_]*(?::[0-5]?[0-9])+(?:\.[0-9_]*)?$`)

func yamlScalar(node *yaml.Node) (any, error) {
	value := node.Value
	explicit := node.Style&yaml.TaggedStyle != 0
	quoted := node.Style&(yaml.SingleQuotedStyle|yaml.DoubleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0
	if node.Tag == "!!str" && (explicit || quoted) {
		return value, nil
	}
	if !explicit && !quoted && node.Tag == "!!float" && !pyYAMLFloat.MatchString(value) {
		return value, nil
	}
	if !explicit && !quoted && yamlBool.MatchString(value) {
		return value == "yes" || value == "Yes" || value == "YES" || value == "true" || value == "True" || value == "TRUE" || value == "on" || value == "On" || value == "ON", nil
	}
	if !explicit && !quoted && yamlSexagesimal.MatchString(value) {
		parts := strings.Split(strings.ReplaceAll(strings.TrimLeft(value, "+-"), "_", ""), ":")
		number := 0.0
		for _, part := range parts {
			v, _ := strconv.ParseFloat(part, 64)
			number = number*60 + v
		}
		if strings.HasPrefix(value, "-") {
			number = -number
		}
		if strings.Contains(value, ".") {
			return number, nil
		}
		return json.Number(strconv.FormatFloat(number, 'f', 0, 64)), nil
	}
	switch node.Tag {
	case "!!str":
		return value, nil
	case "!!null":
		return nil, nil
	case "!!bool":
		if !yamlBool.MatchString(value) {
			return nil, fmt.Errorf("invalid YAML bool")
		}
		return strings.EqualFold(value, "true") || strings.EqualFold(value, "yes") || strings.EqualFold(value, "on"), nil
	case "!!int":
		text := strings.ReplaceAll(value, "_", "")
		base := 10
		digits := strings.TrimLeft(text, "+-")
		sign := ""
		if len(text) > len(digits) {
			sign = text[:1]
		}
		switch {
		case strings.HasPrefix(digits, "0x"):
			base = 16
			digits = digits[2:]
		case strings.HasPrefix(digits, "0b"):
			base = 2
			digits = digits[2:]
		case strings.HasPrefix(digits, "0o"):
			// YAML 1.1's implicit resolver does not recognise YAML 1.2 octal.
			if !explicit {
				return value, nil
			}
			base = 8
			digits = digits[2:]
		case len(digits) > 1 && digits[0] == '0':
			base = 8
		}
		n, ok := new(big.Int).SetString(sign+digits, base)
		if !ok {
			if !explicit {
				return value, nil
			}
			return nil, fmt.Errorf("invalid YAML integer")
		}
		return json.Number(n.String()), nil
	case "!!float":
		text := strings.ToLower(strings.ReplaceAll(value, "_", ""))
		switch text {
		case ".nan":
			return math.NaN(), nil
		case ".inf", "+.inf":
			return math.Inf(1), nil
		case "-.inf":
			return math.Inf(-1), nil
		}
		n, err := strconv.ParseFloat(text, 64)
		return n, err
	case "!!timestamp":
		// YAML date or naive datetime lacks the required timezone. Keep it as text
		// so ConceptFrontmatter's timezone validator rejects it.
		var t time.Time
		if err := node.Decode(&t); err != nil {
			return nil, err
		}
		if len(value) <= 10 || !strings.ContainsAny(value, "Zz+") && !strings.Contains(value[10:], "-") {
			return value, nil
		}
		return t, nil
	case "!!binary":
		raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(value), ""))
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(raw) {
			return recursiveYAML{}, nil
		}
		return string(raw), nil
	default:
		return nil, fmt.Errorf("unsupported YAML tag")
	}
}
