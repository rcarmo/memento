package access

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"

	"golang.org/x/crypto/scrypt"
)

type AccessError struct{ Message string }

func (e *AccessError) Error() string   { return e.Message }
func accessError(message string) error { return &AccessError{Message: message} }
func masterKey(value string) ([]byte, error) {
	if value == "" {
		return nil, accessError("MEMENTO_ADMIN_MASTER_KEY is required")
	}
	return scrypt.Key([]byte(value), []byte("memento-access-v1"), 1<<14, 8, 1, 32)
}
func sealKey(verifier []byte, master string, random io.Reader) (string, error) {
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(random, nonce); err != nil {
		return "", err
	}
	key, err := masterKey(master)
	if err != nil {
		return "", err
	}
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	payload := append(nonce, gcm.Seal(nil, nonce, verifier, []byte("memento-access"))...)
	return base64.URLEncoding.EncodeToString(payload), nil
}
func openKey(payload, master string) ([]byte, error) {
	invalid := func() ([]byte, error) { return nil, accessError("access master key is invalid") }
	// Python's urlsafe decoder ignores non-alphabet bytes and accepts either
	// alphabet, but still requires padding. Do not expose decryption diagnostics.
	var filtered strings.Builder
	for _, r := range payload {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("-_+/=", r) {
			filtered.WriteRune(r)
		}
	}
	raw, err := base64.URLEncoding.DecodeString(strings.NewReplacer("+", "-", "/", "_").Replace(filtered.String()))
	if err != nil || len(raw) < 12 {
		return invalid()
	}
	key, err := masterKey(master)
	if err != nil {
		return invalid()
	}
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, raw[:12], raw[12:], []byte("memento-access"))
	if err != nil {
		return invalid()
	}
	return plain, nil
}
func digest(key []byte, token string) string {
	hash := hmac.New(sha256.New, key)
	_, _ = hash.Write([]byte(token))
	return fmt.Sprintf("%x", hash.Sum(nil))
}
func newToken(random io.Reader) (string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", err
	}
	return "memento_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

var principalName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

func validatePolicy(name string, roles, reads, writes []string) ([]string, []string, []string, error) {
	if !principalName.MatchString(name) {
		return nil, nil, nil, accessError("principal name must use lowercase letters, digits, and hyphens")
	}
	roles = unique(roles)
	reads = unique(reads)
	writes = unique(writes)
	if len(roles) == 0 {
		return nil, nil, nil, accessError("principal roles are invalid")
	}
	for _, role := range roles {
		if !contains([]string{"reader", "proposer", "curator", "admin"}, role) {
			return nil, nil, nil, accessError("principal roles are invalid")
		}
	}
	for _, prefix := range append(append([]string{}, reads...), writes...) {
		if !strings.HasPrefix(prefix, "/") || !strings.HasSuffix(prefix, "/") {
			return nil, nil, nil, accessError("namespace prefixes must start and end with '/'")
		}
	}
	for _, write := range writes {
		found := false
		for _, read := range reads {
			if strings.HasPrefix(write, read) {
				found = true
			}
		}
		if !found {
			return nil, nil, nil, accessError("every write prefix must be inside a readable prefix")
		}
	}
	return roles, reads, writes, nil
}
