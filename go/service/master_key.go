package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"os"
)

type rotateMasterKeyOps struct {
	connect func(context.Context, string) (*sql.DB, error)
	migrate func(context.Context, *sql.DB) error
	rotate  func(context.Context, *sql.DB, string, string) error
	lookup  func(string) (string, bool)
}

func defaultRotateMasterKeyOps() rotateMasterKeyOps {
	return rotateMasterKeyOps{control.Connect, control.Migrate, func(ctx context.Context, db *sql.DB, oldKey, newKey string) error {
		store, err := access.OpenStore(ctx, db, oldKey)
		if err != nil {
			return err
		}
		return store.RotateMasterKey(ctx, oldKey, newKey)
	}, os.LookupEnv}
}
func RotateMasterKey(ctx context.Context, config RuntimeConfig) error {
	return rotateMasterKey(ctx, config, defaultRotateMasterKeyOps())
}
func rotateMasterKey(ctx context.Context, config RuntimeConfig, ops rotateMasterKeyOps) (err error) {
	paths := RuntimePathsFor(config)
	db, err := ops.connect(ctx, paths.ControlDB)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	if err = ops.migrate(ctx, db); err != nil {
		return err
	}
	oldKey, _ := ops.lookup("MEMENTO_ADMIN_PREVIOUS_MASTER_KEY")
	newKey, _ := ops.lookup("MEMENTO_ADMIN_MASTER_KEY")
	return ops.rotate(ctx, db, oldKey, newKey)
}
