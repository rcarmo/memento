package service

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type routerExpansionFixture struct {
	Expansions []struct {
		Input, Request string
		Expansion      map[string]any `json:"expansion"`
	} `json:"expansions"`
}

func TestRouterExpansionFixture(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/router.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture routerExpansionFixture
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, item := range fixture.Expansions {
		action, err := ParseNeedleRouterOutput(item.Input)
		if err != nil {
			t.Fatal(err)
		}
		got := ExpandRouterAction(action, item.Request)
		normalize := func(value any) any {
			raw, _ := json.Marshal(value)
			var out any
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			_ = decoder.Decode(&out)
			return out
		}
		if !reflect.DeepEqual(normalize(got), normalize(item.Expansion)) {
			t.Fatal(item.Request, got, item.Expansion)
		}
	}
}
func TestDerivedSearchQuery(t *testing.T) {
	for input, want := range map[string]string{"find ": "find ", "PLEASE SEARCH FOR x": "x", "kindly fetch x": "x", "locate x": "x", "plain": "plain"} {
		if got := derivedSearchQuery(input); got != want {
			t.Fatal(input, got, want)
		}
	}
	if got := ExpandRouterAction(RouterAction{Action: "bad"}, "x"); got != nil {
		t.Fatal(got)
	}
}
