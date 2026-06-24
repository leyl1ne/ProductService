package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	storageDSN := os.Getenv("POSTGRES_DSN")
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	migrationsTable := os.Getenv("MIGRATIONS_TABLE")

	flag.Parse()

	if storageDSN == "" {
		panic("storage-path is required")
	}
	if migrationsPath == "" {
		panic("migrations-path is required")
	}
	if migrationsTable == "" {
		migrationsTable = "migrations"
	}

	dsn := storageDSN
	if strings.Contains(dsn, "?") {
		dsn += "&"
	} else {
		dsn += "?"
	}
	dsn += "x-migrations-table=" + migrationsTable

	m, err := migrate.New(
		"file://"+migrationsPath,
		dsn,
	)
	if err != nil {
		panic(err)
	}

	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("no migrations to apply")

			return
		}

		panic(err)
	}

}
