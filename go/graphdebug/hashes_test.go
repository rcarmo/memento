package graphdebug

import (
	"context"
	"reflect"
	"testing"
)

func TestContentHashes(t *testing.T) {
	_, path := nodeDB(t)
	s := NewSnapshotService("", path, "")
	got, err := s.ContentHashes(context.Background(), []string{"6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e", "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d"})
	if err != nil || !reflect.DeepEqual(got, map[string]string{"5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d": "ha", "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e": "hb"}) {
		t.Fatal(got, err)
	}
	got, err = s.ContentHashes(context.Background(), nil)
	if err != nil || len(got) != 2 {
		t.Fatal(got, err)
	}
	got, err = s.ContentHashes(context.Background(), []string{})
	if err != nil || got == nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}
