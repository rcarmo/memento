package control

import (
	"encoding/json"
	"testing"
)

func FuzzControlRecords(f *testing.F) {
	for _, raw := range []string{"", "null", `{}`, `{"a":1}`, `[]`, `{"nested":[true,null,"x"]}`} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		request := OperationRequest{RequestJSON: raw}
		firstHash := request.RequestHash()
		if firstHash == "" || firstHash != (OperationRequest{RequestJSON: raw}).RequestHash() {
			t.Fatal("unstable request hash")
		}
		record := OperationRecord{ResultJSON: &raw}
		payload, err := record.ReplayPayload()
		var decoded any
		decodeErr := json.Unmarshal([]byte(raw), &decoded)
		_, object := decoded.(map[string]any)
		if err == nil && decodeErr == nil && object && payload == nil {
			t.Fatal("object result lost")
		}
		proposal := ProposalRecord{PatchJSON: raw}
		patch, patchErr := proposal.Patch()
		asset := ProposalAssetRecord{ProposalAssetInput: ProposalAssetInput{ManifestJSON: raw}}
		manifest, manifestErr := asset.Manifest()
		if (patchErr == nil) != object || (manifestErr == nil) != object {
			t.Fatalf("object validation mismatch: object=%t patch=%v manifest=%v", object, patchErr, manifestErr)
		}
		if object && (patch == nil || manifest == nil) {
			t.Fatal("decoded object lost")
		}
	})
}

func FuzzControlCompatibilityStrings(f *testing.F) {
	for _, value := range []string{"", "hello-world", "HTTP_server", "éCLAIR", "\x00", "two  words"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 1<<20 {
			t.Skip()
		}
		_ = pythonTitle(value)
		if pyString([]byte(value)) != value || pyString(value) != value || pyString(nil) != "None" {
			t.Fatal("Python string compatibility mismatch")
		}
	})
}
