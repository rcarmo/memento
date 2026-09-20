package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"time"
)

// Fernet v0x80: 16-byte signing key, 16-byte AES key, authenticated timestamp/IV
// and PKCS7-padded CBC ciphertext. Proposal cursors have no TTL in the source;
// their principal/policy/revision scope provides invalidation. Keys stay in RAM.
type proposalCursor struct{ key [32]byte }

var errProposalCursor = errors.New("invalid or stale proposal list cursor")

func (p proposalCursor) encrypt(data []byte, now time.Time, random io.Reader) (string, error) {
	var iv [aes.BlockSize]byte
	if _, err := io.ReadFull(random, iv[:]); err != nil {
		return "", err
	}
	padding := aes.BlockSize - len(data)%aes.BlockSize
	plain := make([]byte, len(data)+padding)
	copy(plain, data)
	for i := len(data); i < len(plain); i++ {
		plain[i] = byte(padding)
	}
	token := make([]byte, 25+len(plain)+sha256.Size)
	token[0] = 0x80
	binary.BigEndian.PutUint64(token[1:9], uint64(now.Unix()))
	copy(token[9:25], iv[:])
	block, _ := aes.NewCipher(p.key[16:])
	cipher.NewCBCEncrypter(block, iv[:]).CryptBlocks(token[25:len(token)-sha256.Size], plain)
	mac := hmac.New(sha256.New, p.key[:16])
	_, _ = mac.Write(token[:len(token)-sha256.Size])
	copy(token[len(token)-sha256.Size:], mac.Sum(nil))
	return base64.URLEncoding.EncodeToString(token), nil
}
func (p proposalCursor) decrypt(text string) ([]byte, error) {
	// urlsafe_b64decode accepts ASCII non-alphabet characters, standard +/ and
	// redundant trailing padding. Preserve that decode boundary before HMAC.
	var filtered strings.Builder
	for _, r := range text {
		if r > 127 {
			return nil, errProposalCursor
		}
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '=', r == '-', r == '_':
			filtered.WriteByte(byte(r))
		case r == '+':
			filtered.WriteByte('-')
		case r == '/':
			filtered.WriteByte('_')
		}
	}
	encoded := filtered.String()
	if i := strings.IndexByte(encoded, '='); i >= 0 {
		if strings.Trim(encoded[i:], "=") != "" {
			return nil, errProposalCursor
		}
		unpadded := i % 4
		if unpadded == 1 {
			return nil, errProposalCursor
		}
		needed := (4 - unpadded) % 4
		if len(encoded)-i < needed {
			return nil, errProposalCursor
		}
		encoded = encoded[:i] + strings.Repeat("=", needed)
	}
	token, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil || len(token) < 25+aes.BlockSize+sha256.Size || token[0] != 0x80 {
		return nil, errProposalCursor
	}
	body, signature := token[:len(token)-sha256.Size], token[len(token)-sha256.Size:]
	mac := hmac.New(sha256.New, p.key[:16])
	_, _ = mac.Write(body)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, errProposalCursor
	}
	encrypted := body[25:]
	if len(encrypted)%aes.BlockSize != 0 {
		return nil, errProposalCursor
	}
	plain := make([]byte, len(encrypted))
	block, _ := aes.NewCipher(p.key[16:])
	cipher.NewCBCDecrypter(block, body[9:25]).CryptBlocks(plain, encrypted)
	padding := int(plain[len(plain)-1])
	if padding < 1 || padding > aes.BlockSize {
		return nil, errProposalCursor
	}
	for _, b := range plain[len(plain)-padding:] {
		if int(b) != padding {
			return nil, errProposalCursor
		}
	}
	return plain[:len(plain)-padding], nil
}
