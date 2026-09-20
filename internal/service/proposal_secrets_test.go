package service

import (
	"math"
	"testing"
)

func TestProposalSecretScanner(t *testing.T) {
	bad := []string{"AKIA1234567890ABCDEF", "ghp_12345678901234567890", "api_key=1234567890abcdef", "token: 1234567890abcdef", "-----BEGIN RSA PRIVATE KEY-----", "0123456789abcdefABCDEF+/=_-"}
	for _, value := range bad {
		if err := ScanProposalChangeSecrets(map[string]any{"kind": "patch", "path": "/a", "body": value}, 16); err == nil {
			t.Fatal(value)
		}
	}
	for _, key := range []string{"path", "title", "description"} {
		if err := ScanProposalChangeSecrets(map[string]any{key: "ghp_12345678901234567890"}, 32); err == nil {
			t.Fatal(key)
		}
	}
	if err := ScanProposalChangeSecrets(map[string]any{"body": "ordinary prose", "other": "ghp_12345678901234567890"}, 32); err != nil {
		t.Fatal(err)
	}
	if containsProposalSecret("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 32) {
		t.Fatal("low entropy")
	}
	if entropy := shannonEntropy("aaaa"); entropy != 0 {
		t.Fatal(entropy)
	}
	if entropy := shannonEntropy("ab"); math.Abs(entropy-1) > .0001 {
		t.Fatal(entropy)
	}
}
