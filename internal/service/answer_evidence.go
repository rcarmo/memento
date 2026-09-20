package service

import (
	"regexp"
	"strings"
)

type QueryProfile struct {
	SecretIntent   bool     `json:"secret_intent"`
	TemporalIntent string   `json:"temporal_intent"`
	Relational     bool     `json:"relational"`
	NamespaceHint  *string  `json:"namespace_hint"`
	Terms          []string `json:"terms"`
}

var answerWords = regexp.MustCompile(`\pL[\pL\pN_]*|\pN+`)
var answerStop = wordSet("a about an and are at be by can did do does for from how in is it me of on please should tell the this to was were what when where which who why will with would")
var secretPhrases = []string{"api key", "credential", "credentials", "password", "passphrase", "private key", "recovery code", "rotation code", "secret", "access token", "api token", "auth token", "bearer token", "token value"}
var historicalPhrases = []string{"before ", "formerly", "historical", "history", "previously", "prior to", "used to", "why was"}
var relationalPhrases = []string{"connected to", "connected with", "depend on", "depends on", "linked to", "managed by", "manages", "owned by", "owns", "powered by", "powers", "rack that", "related to", "reports to", "which host", "which rack", "which service", "who manages", "who owns"}

func wordSet(raw string) map[string]bool {
	m := map[string]bool{}
	for _, v := range strings.Fields(raw) {
		m[v] = true
	}
	return m
}
func containsPhrase(raw string, phrases []string) bool {
	for _, v := range phrases {
		if strings.Contains(raw, v) {
			return true
		}
	}
	return false
}
func ProfileQuestion(question string) QueryProfile {
	normalized := NormalizeQuestion(question)
	words := answerWords.FindAllString(normalized, -1)
	seen := map[string]bool{}
	terms := []string{}
	for _, w := range words {
		if !answerStop[w] && !seen[w] {
			seen[w] = true
			terms = append(terms, w)
		}
	}
	temporal := "neutral"
	if containsPhrase(normalized, historicalPhrases) {
		temporal = "historical"
	} else {
		current := wordSet("accepted current currently latest now recent recently today")
		for _, w := range words {
			if current[w] {
				temporal = "current"
				break
			}
		}
	}
	var namespace *string
	for _, w := range words {
		if w == "personal" {
			v := "personal"
			namespace = &v
			break
		}
		if w == "work" || w == "workspace" || w == "company" {
			v := "work"
			namespace = &v
			break
		}
	}
	return QueryProfile{containsPhrase(normalized, secretPhrases), temporal, containsPhrase(normalized, relationalPhrases), namespace, terms}
}
func NamespaceMatches(p QueryProfile, path string) bool {
	if p.NamespaceHint == nil {
		return true
	}
	return strings.HasPrefix(path, "/"+*p.NamespaceHint+"/")
}
func SensitiveEvidence(tags []string) bool {
	for _, v := range tags {
		v = strings.ToLower(v)
		if v == "credential" || v == "credentials" || v == "secret" || v == "secrets" {
			return true
		}
	}
	return false
}
func CurrentlyIneligible(status string, tags []string) bool {
	if status == "deprecated" || status == "tombstone" {
		return true
	}
	for _, v := range tags {
		v = strings.ToLower(v)
		if v == "conflicting" || v == "historical" || v == "obsolete" || v == "stale" {
			return true
		}
	}
	return false
}
func EvidenceTextMatches(p QueryProfile, title, body string) bool {
	if len(p.Terms) == 0 {
		return false
	}
	set := map[string]bool{}
	words := answerWords.FindAllString(strings.ToLower(title+" "+body), -1)
	for _, w := range words {
		set[w] = true
	}
	matched := 0
	for _, term := range p.Terms {
		ok := set[term]
		if !ok && len([]rune(term)) >= 5 {
			prefix := string([]rune(term)[:5])
			for w := range set {
				if strings.HasPrefix(w, prefix) {
					ok = true
					break
				}
			}
		}
		if ok {
			matched++
		}
	}
	required := 2
	if len(p.Terms) == 1 {
		required = 1
	}
	return matched >= required
}
func EvidenceSufficient(p QueryProfile, concepts []AnswerReadConcept) bool {
	if len(concepts) == 0 {
		return false
	}
	matched := false
	namespace := p.NamespaceHint == nil
	historical, current := false, false
	for _, c := range concepts {
		matched = matched || EvidenceTextMatches(p, c.Title, c.Body)
		namespace = namespace || NamespaceMatches(p, c.Path)
		historical = historical || c.Status == "deprecated" || hasAnyTag(c.Tags, "historical", "temporal")
		current = current || (c.Status == "active" && !CurrentlyIneligible(c.Status, c.Tags))
	}
	if !matched || !namespace {
		return false
	}
	if p.TemporalIntent == "historical" {
		return historical
	}
	if p.TemporalIntent == "current" {
		return current
	}
	return true
}
func hasAnyTag(tags []string, wants ...string) bool {
	for _, tag := range tags {
		for _, want := range wants {
			if tag == want {
				return true
			}
		}
	}
	return false
}
