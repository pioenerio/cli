package migrate

import (
	"database/sql"

	"github.com/dnote/cli/infra"
	"github.com/dnote/cli/log"
	"github.com/pkg/errors"
)

// Sequence is a list of migrations to be run
var Sequence = []migration{
	m1,
	m2,
	m3,
	m4,
	m5,
	m6,
}

func initSchema(ctx infra.DnoteCtx) (int, error) {
	schemaVersion := 1

	db := ctx.DB
	_, err := db.Exec("INSERT INTO system (key, value) VALUES (? ,?)", "schema", schemaVersion)
	if err != nil {
		return schemaVersion, errors.Wrap(err, "inserting schema")
	}

	return schemaVersion, nil
}

func getSchema(ctx infra.DnoteCtx) (int, error) {
	var ret int

	db := ctx.DB
	err := db.QueryRow("SELECT value FROM system where key = ?", "schema").Scan(&ret)
	if err == sql.ErrNoRows {
		ret, err = initSchema(ctx)

		if err != nil {
			return ret, errors.Wrap(err, "initializing schema")
		}
	} else if err != nil {
		return ret, errors.Wrap(err, "querying schema")
	}

	return ret, nil
}

func execute(ctx infra.DnoteCtx, m migration) error {
	log.Debug("running migration %s\n", m.name)

	tx, err := ctx.DB.Begin()
	if err != nil {
		return errors.Wrap(err, "beginning a transaction")
	}

	err = m.run(tx)
	if err != nil {
		tx.Rollback()
		return errors.Wrapf(err, "running migration '%s'", m.name)
	}

	var currentSchema int
	err = tx.QueryRow("SELECT value FROM system WHERE key = ?", "schema").Scan(&currentSchema)
	if err != nil {
		tx.Rollback()
		return errors.Wrap(err, "getting current schema")
	}

	_, err = tx.Exec("UPDATE system SET value = ? WHERE key = ?", currentSchema+1, "schema")
	if err != nil {
		tx.Rollback()
		return errors.Wrap(err, "incrementing schema")
	}

	tx.Commit()

	return nil
}

// Run performs unrun migrations
func Run(ctx infra.DnoteCtx, migrations []migration) error {
	schema, err := getSchema(ctx)
	if err != nil {
		return errors.Wrap(err, "getting the current schema")
	}

	log.Debug("current schema %d\n", schema)

	if schema == len(migrations) {
		return nil
	}

	toRun := migrations[schema:]

	for _, m := range toRun {
		if err := execute(ctx, m); err != nil {
			return errors.Wrapf(err, "running migration %s", m.name)
		}
	}

	return nil
}
