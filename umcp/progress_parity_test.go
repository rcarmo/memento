package umcp

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSyncAsyncProgressParity(t *testing.T) {
	data, err := os.ReadFile("testdata/parity/umcp-progress.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input struct {
			Token    json.RawMessage
			Progress json.RawMessage
			Total    json.RawMessage
			Message  *string
		}
		Expected json.RawMessage
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		var notifications = []any{}
		ctx := context.WithValue(context.Background(), runtimeKey{}, requestRuntime{progressToken: c.Input.Token, notify: func(method string, params map[string]any) error {
			notifications = append(notifications, map[string]any{"method": method, "params": params})
			return nil
		}})
		p, _ := decode(c.Input.Progress)
		total, _ := decode(c.Input.Total)
		err := NotifyProgress(ctx, p, total, c.Input.Message)
		var message any
		if err != nil {
			message = err.Error()
		}
		raw, e := json.Marshal(map[string]any{"notifications": notifications, "error": message})
		if e != nil {
			t.Fatal(e)
		}
		got, _ := decode(raw)
		want, _ := decode(c.Expected)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("progress %s total %s: got %s want %s", c.Input.Progress, c.Input.Total, raw, c.Expected)
		}
	}
}
