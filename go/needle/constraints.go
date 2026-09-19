package needle

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type trie struct {
	children map[rune]*trie
	terminal bool
}

func (t *trie) insert(word string) {
	node := t
	for _, ch := range word {
		if node.children == nil {
			node.children = make(map[rune]*trie)
		}
		if node.children[ch] == nil {
			node.children[ch] = &trie{}
		}
		node = node.children[ch]
	}
	node.terminal = true
}
func (t *trie) node(prefix string) *trie {
	for _, ch := range prefix {
		t = t.children[ch]
		if t == nil {
			return nil
		}
	}
	return t
}
func tokenValid(text string, node *trie) bool {
	for _, ch := range text {
		if ch == '"' {
			return node.terminal
		}
		node = node.children[ch]
		if node == nil {
			return false
		}
	}
	return true
}

type jsonState int

const (
	free jsonState = iota
	inName
	inArgKey
)

type stateMachine struct {
	state                                jsonState
	buffer, constrained, currentFunction string
	inArguments, inString, escaped       bool
	argumentsDepth, nesting              int
}

func (s *stateMachine) feed(text string) {
	for _, ch := range text {
		s.feedRune(ch)
	}
}
func (s *stateMachine) feedRune(ch rune) {
	if s.state == inName || s.state == inArgKey {
		if ch == '"' {
			if s.state == inName {
				s.currentFunction = s.constrained
			}
			s.constrained = ""
			s.state = free
		} else {
			s.constrained += string(ch)
		}
		s.buffer += string(ch)
		return
	}
	s.buffer += string(ch)
	if s.inString {
		if s.escaped {
			s.escaped = false
			return
		}
		if ch == '\\' {
			s.escaped = true
			return
		}
		if ch == '"' {
			s.inString = false
		}
		return
	}
	if ch == '{' || ch == '[' {
		s.nesting++
	}
	if ch == '}' || ch == ']' {
		s.nesting = max(0, s.nesting-1)
		if ch == '}' && s.inArguments && s.nesting < s.argumentsDepth {
			s.inArguments = false
		}
		return
	}
	if strings.HasSuffix(s.buffer, `"name":"`) && !s.inArguments {
		s.state = inName
		return
	}
	if strings.HasSuffix(s.buffer, `"arguments":{`) {
		s.inArguments = true
		s.argumentsDepth = s.nesting
		return
	}
	if s.inArguments && s.nesting == s.argumentsDepth && (strings.HasSuffix(s.buffer, `{"`) || strings.HasSuffix(s.buffer, `,"`)) {
		s.state = inArgKey
		return
	}
	if ch == '"' {
		prefix := strings.TrimRightFunc(strings.TrimSuffix(s.buffer, `"`), unicode.IsSpace)
		if strings.HasSuffix(prefix, ":") {
			s.inString = true
		}
	}
}

type constraintTemplate struct {
	strings []string
	names   trie
	params  map[string]*trie
}
type constraints struct {
	machine  stateMachine
	template *constraintTemplate
}

func newConstraintTemplate(tools string, t *Tokenizer) *constraintTemplate {
	c := &constraintTemplate{params: make(map[string]*trie), strings: make([]string, t.VocabSize())}
	for i := range c.strings {
		c.strings[i] = t.TokenString(i)
	}
	if value, ok := parseJSON(tools); ok {
		if items, ok := value.([]any); ok {
			for _, item := range items {
				object, ok := item.(map[string]any)
				if !ok {
					continue
				}
				name, ok := object["name"].(string)
				if !ok {
					continue
				}
				c.names.insert(name)
				parameters := &trie{}
				if props, ok := object["parameters"].(map[string]any); ok {
					for key, v := range props {
						if _, ok := v.(map[string]any); ok {
							parameters.insert(key)
						}
					}
				}
				c.params[name] = parameters
			}
		}
	}
	return c
}
func newConstraints(tools string, t *Tokenizer) *constraints {
	return &constraints{template: newConstraintTemplate(tools, t)}
}
func (c *constraints) update(token int) {
	if token >= 0 && token < len(c.template.strings) {
		c.machine.feed(c.template.strings[token])
	}
}
func (c *constraints) allowed() []int { return c.allowedInto(nil) }
func (c *constraints) allowedInto(allowed []int) []int {
	allowed = allowed[:0]
	var root *trie
	switch c.machine.state {
	case free:
		return nil
	case inName:
		root = &c.template.names
	case inArgKey:
		root = c.template.params[c.machine.currentFunction]
		if root == nil {
			return nil
		}
	}
	node := root.node(c.machine.constrained)
	if node == nil {
		return nil
	}
	for id, text := range c.template.strings {
		first, size := utf8.DecodeRuneInString(text)
		if size == 0 {
			continue
		}
		if node.children[first] != nil || first == '"' && node.terminal {
			if tokenValid(text, node) {
				allowed = append(allowed, id)
			}
		}
	}
	return allowed
}
