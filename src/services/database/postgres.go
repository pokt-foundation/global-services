package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	ErrValueUnderZero     = errors.New("value must be lower or equal to zero")
	ErrMaxPoolSizeOverMin = errors.New("max pool size should be more than min pool size")
)

type PostgresOptions struct {
	Connection         string
	MinMetricsPoolSize int
	MaxMetricsPoolSize int
}

type Postgres struct {
	Conn *pgxpool.Pool
}

func NewPostgresDatabase(ctx context.Context, options *PostgresOptions) (*Postgres, error) {
	if options.MinMetricsPoolSize < 0 || options.MaxMetricsPoolSize < 0 {
		return nil, ErrValueUnderZero
	}

	if options.MinMetricsPoolSize > options.MaxMetricsPoolSize {
		return nil, ErrMaxPoolSizeOverMin
	}

	config, err := pgxpool.ParseConfig(fmt.Sprintf("%s?pool_min_conns=%d&pool_max_conns=%d", options.Connection, options.MinMetricsPoolSize, options.MaxMetricsPoolSize))
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
