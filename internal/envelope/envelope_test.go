package envelope

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestPythonParity(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/envelopes.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name  string
		Kind  string
		Input struct {
			Data          any
			ErrorClass    string `json:"error_class"`
			Message       string
			RepoRevision  *string `json:"repo_revision"`
			IndexRevision *string `json:"index_revision"`
			IndexStale    bool    `json:"index_stale"`
			OperationID   *string `json:"operation_id"`
			Warnings      []string
			NextTools     []string `json:"next_tools"`
		}
		Expected any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var actual any
			if c.Kind == "success" {
				v := NewSuccess(c.Input.Data, *c.Input.RepoRevision, *c.Input.IndexRevision)
				v.IndexStale = c.Input.IndexStale
				v.OperationID = c.Input.OperationID
				if c.Input.Warnings != nil {
					v.Warnings = c.Input.Warnings
				}
				if c.Input.NextTools != nil {
					v.NextTools = c.Input.NextTools
				}
				actual = v
			} else {
				v, err := NewFailure(c.Input.ErrorClass, c.Input.Message)
				if err != nil {
					t.Fatal(err)
				}
				v.RepoRevision = c.Input.RepoRevision
				v.IndexRevision = c.Input.IndexRevision
				v.IndexStale = c.Input.IndexStale
				v.OperationID = c.Input.OperationID
				if c.Input.Warnings != nil {
					v.Warnings = c.Input.Warnings
				}
				actual = v
			}
			encoded, err := json.Marshal(actual)
			if err != nil {
				t.Fatal(err)
			}
			var got any
			if err = json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.Expected) {
				t.Fatalf("got %s; expected %#v", encoded, c.Expected)
			}
		})
	}
}

func TestFailureValidation(t *testing.T) {
	for _, c := range [][2]string{{"", "message"}, {"class", ""}, {"", ""}} {
		if _, err := NewFailure(c[0], c[1]); err == nil {
			t.Fatal("accepted empty error field")
		}
	}
}
