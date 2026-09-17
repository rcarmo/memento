package umcp

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestPaginationPythonParity(t *testing.T) {
	data, err := os.ReadFile("../testdata/parity/umcp-pagination.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input struct {
			Items       []map[string]string
			Label       string
			ResultKey   string `json:"result_key"`
			Principal   string
			DefaultSize int `json:"default_size"`
			Params      json.RawMessage
		}
		Expected json.RawMessage
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		items := []DiscoveryItem{}
		for _, item := range c.Input.Items {
			items = append(items, DiscoveryItem{Identity: []string{item["name"]}, Value: item})
		}
		params, _ := decode(c.Input.Params)
		ctx := context.WithValue(context.Background(), requestContextKey{}, RequestContext{Principal: c.Input.Principal})
		result, rpcErr := ListPage(ctx, c.Input.Label, c.Input.ResultKey, items, params.(map[string]any), c.Input.DefaultSize)
		raw, err := json.Marshal(map[string]any{"result": result, "error": rpcErr})
		if err != nil {
			t.Fatal(err)
		}
		got, _ := decode(raw)
		want, _ := decode(c.Expected)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("params %s default %d\ngot %s\nwant %s", c.Input.Params, c.Input.DefaultSize, raw, c.Expected)
		}
	}
}

func FuzzPagination(f *testing.F) {
	for _, s := range []string{"", "a", "null", "!!!!", "eyJ2IjoxfQ"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, cursor string) {
		_, _ = ListPage(context.Background(), "tools", "tools", []DiscoveryItem{{Identity: []string{"name"}, Value: map[string]string{"name": "name"}}}, map[string]any{"cursor": cursor}, 0)
	})
}
