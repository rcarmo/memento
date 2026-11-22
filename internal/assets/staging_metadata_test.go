package assets

import (
	"archive/zip"
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"
)

func TestStagingMetadataReceipt(t *testing.T) {
	store, clock := stagingStore(t)
	ctx := context.Background()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("readme.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = entry.Write([]byte("immutable asset")); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	staged, _, err := store.Put(ctx, "agent", "metadata", "asset", "1.0.0", buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	metadata, size, err := store.Metadata(ctx, "agent", staged.StagedAssetID)
	if err != nil || size != int64(buffer.Len()) || len(metadata.BlobBytes) != 0 || !reflect.DeepEqual(metadata.PublicPayload(), staged.PublicPayload()) {
		t.Fatal(metadata, size, err)
	}
	if _, _, err = store.Metadata(ctx, "other", staged.StagedAssetID); err == nil {
		t.Fatal("principal binding")
	}
	if _, _, err = store.Metadata(ctx, "agent", "missing"); err == nil {
		t.Fatal("missing receipt")
	}
	// Staging expiry deletes the bytes; a stored SHA alone cannot confirm them.
	*clock = clock.Add(25 * time.Hour)
	metadata, size, err = store.Metadata(ctx, "agent", staged.StagedAssetID)
	if err != nil || size != 0 || metadata.State != "expired" {
		t.Fatal(metadata, size, err)
	}
	if err = store.DB.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.Metadata(ctx, "agent", staged.StagedAssetID); err == nil {
		t.Fatal("closed storage")
	}
}
