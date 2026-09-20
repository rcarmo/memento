package service

import (
	"errors"
	"math"
	"regexp"
)

var proposalSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`(?i)gh[pousr]_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`(?i)(?:api|secret|access)[_-]?key\s*[:=]\s*[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`(?i)(?:token|bearer)\s*[:=]\s*[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`(?i)-----BEGIN [A-Z ]+PRIVATE KEY-----`),
}
var proposalEntropyToken = regexp.MustCompile(`[A-Za-z0-9+/=_-]{16,}`)

func ScanProposalChangeSecrets(change map[string]any, maxEntropyChars int) error {
	values := []string{}
	for _, key := range []string{"path", "title", "body", "description"} {
		if value, ok := change[key].(string); ok {
			values = append(values, value)
		}
	}
	for _, value := range values {
		if containsProposalSecret(value, maxEntropyChars) {
			return errors.New("proposal blocked by secret scanner")
		}
	}
	return nil
}
func containsProposalSecret(value string, maxEntropyChars int) bool {
	for _, pattern := range proposalSecretPatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	for _, token := range proposalEntropyToken.FindAllString(value, -1) {
		if len(token) >= maxEntropyChars && shannonEntropy(token) >= 3.5 {
			return true
		}
	}
	return false
}
func shannonEntropy(value string) float64 {
	counts := map[rune]int{}
	length := 0
	for _, char := range value {
		counts[char]++
		length++
	}
	entropy := 0.0
	for _, count := range counts {
		probability := float64(count) / float64(length)
		entropy -= probability * math.Log2(probability)
	}
	return entropy
}
