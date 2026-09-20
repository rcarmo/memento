package service

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"strings"
	"testing"
	"time"
)

func signCursor(p proposalCursor, body []byte) string {
	mac := hmac.New(sha256.New, p.key[:16])
	mac.Write(body)
	return base64.URLEncoding.EncodeToString(append(body, mac.Sum(nil)...))
}
func TestCursorRejectsTamperingAndPadding(t *testing.T) {
	p := testCursor()
	now := time.Unix(1789689600, 0)
	if _, err := p.encrypt(nil, now, bytes.NewReader(nil)); err == nil {
		t.Fatal("IV failure")
	}
	token, err := p.encrypt([]byte("data"), now, bytes.NewReader(make([]byte, 16)))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"💀", "=abc", "a=", "abc=", token[:len(token)-3], token[:8] + "A" + token[9:], "!!!!"} {
		if _, err := p.decrypt(bad); err == nil {
			t.Fatal("invalid token", bad)
		}
	}
	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	version := append([]byte{}, raw...)
	version[0] = 0x81
	if _, err := p.decrypt(base64.URLEncoding.EncodeToString(version)); err == nil {
		t.Fatal("version")
	}
	raw[30] ^= 1
	if _, err := p.decrypt(base64.URLEncoding.EncodeToString(raw)); err == nil {
		t.Fatal("HMAC")
	}
	raw[30] ^= 1
	// Invalid ciphertext length with a valid MAC must fail before CBC panics.
	body := append([]byte{}, raw[:len(raw)-sha256.Size]...)
	body = append(body, 0)
	if _, err := p.decrypt(signCursor(p, body)); err == nil {
		t.Fatal("block alignment")
	}
	for _, padding := range []byte{0, 17, 2} {
		body := make([]byte, 25+aes.BlockSize)
		body[0] = 0x80
		plain := bytes.Repeat([]byte{1}, aes.BlockSize)
		plain[len(plain)-1] = padding
		block, _ := aes.NewCipher(p.key[16:])
		cipher.NewCBCEncrypter(block, body[9:25]).CryptBlocks(body[25:], plain)
		if _, err := p.decrypt(signCursor(p, body)); err == nil {
			t.Fatal("invalid padding", padding)
		}
	}
	// Standard base64 alphabet is accepted by Python's URL-safe decoder too.
	token, err = p.encrypt([]byte("look for plus and slash"), now, bytes.NewReader(bytes.Repeat([]byte{255}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	standard := strings.NewReplacer("-", "+", "_", "/").Replace(token)
	if _, err = p.decrypt(standard); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"a=", "ab=", "abc"} {
		if _, err = p.decrypt(input); err == nil {
			t.Fatal(input)
		}
	}
}
func TestCursorKeyAndJSONFailures(t *testing.T) {
	controls := ProposalControls{Random: bytes.NewReader(nil)}
	if _, err := controls.encodeProposalCursor(nil, ""); err == nil {
		t.Fatal("key source")
	}
	if _, err := controls.decodeProposalCursor("token", nil); err == nil {
		t.Fatal("decode key source")
	}
	codec := testCursor()
	controls.cursor = &codec
	if _, err := controls.encodeProposalCursor(map[string]any{"bad": make(chan int)}, ""); err == nil {
		t.Fatal("JSON")
	}
	for _, body := range []string{"{", `{"scope":{}}`, `{"scope":{},"after":null}`, `{"scope":{},"after":"p"}`} {
		token, err := codec.encrypt([]byte(body), time.Now(), bytes.NewReader(make([]byte, 16)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = controls.decodeProposalCursor(token, map[string]any{"principal": "other"}); err == nil {
			t.Fatal(body)
		}
	}
	controls.Random = bytes.NewReader(make([]byte, 16))
	if _, err := controls.encodeProposalCursor(nil, "p"); err != nil {
		t.Fatal(err)
	}
	if _, err := controls.encodeProposalCursor(nil, "p"); err != io.EOF {
		t.Fatal(err)
	}
}
func FuzzProposalCursor(f *testing.F) {
	codec := testCursor()
	token, _ := codec.encrypt([]byte(`{"scope":{},"after":"p"}`), time.Unix(1, 0), bytes.NewReader(make([]byte, 16)))
	f.Add(token)
	f.Add("bad")
	f.Add("===")
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 8192 {
			return
		}
		plain, err := codec.decrypt(text)
		if err != nil {
			return
		}
		again, err := codec.encrypt(plain, time.Unix(1, 0), bytes.NewReader(make([]byte, 16)))
		if err != nil {
			t.Fatal(err)
		}
		got, err := codec.decrypt(again)
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatal("roundtrip", err)
		}
	})
}
