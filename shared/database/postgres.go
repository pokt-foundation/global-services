package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	// ErrValueUnderZero when a value given is under zero
	ErrValueUnderZero = errors.New("value must be lower or equal to zero")
	// ErrMaxPoolSizeOverMin when the value given for max pool size is over the min pool size
	ErrMaxPoolSizeOverMin = errors.New("max pool size should be more than min pool size")
)

// PostgresOptions represents the options related to set up a pg connection
type PostgresOptions struct {
	Connection  string
	MinPoolSize int
	MaxPoolSize int
}

// Postgres is the struct for performing operations on a postgres database
type Postgres struct {
	Conn *pgxpool.Pool
}

// NewPostgresDatabase attempts and returns a postgres connection on success
func NewPostgresDatabase(ctx context.Context, options *PostgresOptions) (*Postgres, error) {
	if options.MinPoolSize < 0 || options.MaxPoolSize < 0 {
		return nil, ErrValueUnderZero
	}

	if options.MinPoolSize > options.MaxPoolSize {
		return nil, ErrMaxPoolSizeOverMin
	}

	config, err := pgxpool.ParseConfig(fmt.Sprintf("%s?pool_min_conns=%d&pool_max_conns=%d", options.Connection, options.MinPoolSize, options.MaxPoolSize))
	if err != nil {
		return nil, err
	}

	conn, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return &Postgres{
		Conn: conn,
	}, nil
}
