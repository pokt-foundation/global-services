package database

import "github.com/Pocket/global-services/shared/database"

// CherryPickerDB is an interface to operations in the cherry picker database
type CherryPickerPostgres struct {
	Db *database.Postgres
}

func NewCherryPickerPostgres(db *database.Postgres) *CherryPickerPostgres {
	return &CherryPickerPostgres{
		Db: db,
	}
}
