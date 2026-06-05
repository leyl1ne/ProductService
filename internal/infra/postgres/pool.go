package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, cfg Config) (*Postgres, error) {
	const op = "infra.postgres.Open"

	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("%s: parse config: %w", op, err)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("%s: create pool: %w", op, err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("%s: ping: %w", op, err)
	}

	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Pool() *pgxpool.Pool {
	if p == nil {
		return nil
	}

	return p.pool
}

func (p *Postgres) Close() error {
	const op = "infra.postgres.Close"

	if p == nil || p.pool == nil {
		return fmt.Errorf("%s: postgres is not initialized", op)
	}
	p.pool.Close()
	return nil
}
