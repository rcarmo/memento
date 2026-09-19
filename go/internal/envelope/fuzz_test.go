package envelope

import (
	"encoding/json"
	"testing"
)

func FuzzEnvelopes(f *testing.F) {
	for _, seed := range [][3]string{{"class", "message", `{}`}, {"", "message", `null`}, {"class", "", `[]`}, {"é", "\x00", `{"a":1}`}} {
		f.Add(seed[0], seed[1], seed[2])
	}
	f.Fuzz(func(t *testing.T, class, message, raw string) {
		if len(class)+len(message)+len(raw) > 1<<20 {
			t.Skip()
		}
		var data any
		if json.Unmarshal([]byte(raw), &data) != nil {
			data = raw
		}
		success := NewSuccess(data, "repo", "index")
		encoded, err := json.Marshal(success)
		if err != nil || !json.Valid(encoded) || success.Status != "success" || success.Warnings == nil || success.NextTools == nil {
			t.Fatalf("invalid success envelope: %s %v", encoded, err)
		}
		failure, err := NewFailure(class, message)
		if class == "" || message == "" {
			if err == nil {
				t.Fatal("accepted empty failure field")
			}
			return
		}
		if err != nil || failure.Status != "error" || failure.Warnings == nil {
			t.Fatalf("invalid failure envelope: %#v %v", failure, err)
		}
	})
}
