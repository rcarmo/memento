package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	_ "modernc.org/sqlite"
	"path/filepath"
	"testing"
)

func TestRotateMasterKey(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = t.TempDir()
	db, err := control.Connect(ctx, RuntimePathsFor(config).ControlDB)
	if err != nil {
		t.Fatal(err)
	}
	if err = control.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err = access.OpenStore(ctx, db, "old"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	t.Setenv("MEMENTO_ADMIN_PREVIOUS_MASTER_KEY", "old")
	t.Setenv("MEMENTO_ADMIN_MASTER_KEY", "new")
	if err = RotateMasterKey(ctx, config); err != nil {
		t.Fatal(err)
	}
	db, err = control.Connect(ctx, RuntimePathsFor(config).ControlDB)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = access.OpenStore(ctx, db, "new"); err != nil {
		t.Fatal(err)
	}
	if _, err = access.OpenStore(ctx, db, "old"); err == nil {
		t.Fatal("old key")
	}
}
func TestDefaultRotateMasterKeyOpenFailure(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "empty.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = defaultRotateMasterKeyOps().rotate(context.Background(), db, "old", "new"); err == nil {
		t.Fatal("open")
	}
}
func TestRotateMasterKeyFailures(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	for _, stage := range []string{"connect", "migrate", "rotate"} {
		base := rotateMasterKeyOps{connect: func(context.Context, string) (*sql.DB, error) {
			return sql.Open("sqlite", filepath.Join(t.TempDir(), stage+".sqlite"))
		}, migrate: func(context.Context, *sql.DB) error { return nil }, rotate: func(context.Context, *sql.DB, string, string) error { return nil }, lookup: func(name string) (string, bool) {
			return map[string]string{"MEMENTO_ADMIN_PREVIOUS_MASTER_KEY": "old", "MEMENTO_ADMIN_MASTER_KEY": "new"}[name], true
		}}
		switch stage {
		case "connect":
			base.connect = func(context.Context, string) (*sql.DB, error) { return nil, boom }
		case "migrate":
			base.migrate = func(context.Context, *sql.DB) error { return boom }
		case "rotate":
			base.rotate = func(context.Context, *sql.DB, string, string) error { return boom }
		}
		if err := rotateMasterKey(ctx, RuntimeConfig{}, base); !errors.Is(err, boom) {
			t.Fatal(stage, err)
		}
	}
}
