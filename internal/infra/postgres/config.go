package postgres

import "time"

type Config struct {
	DSN             string        `yaml:"dsn"                env:"POSTGRES_DSN"`
	MaxOpenConns    int           `yaml:"max_open_conns"     env:"POSTGRES_MAX_OPEN_CONNS"`
	MaxIdleConns    int           `yaml:"max_idle_conns"     env:"POSTGRES_MAX_IDLE_CONNS"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"  env:"POSTGRES_CONN_MAX_LIFETIME"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" env:"POSTGRES_CONN_MAX_IDLE_TIME"`
}
