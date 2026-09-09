// migrate applies pending schema migrations then exits. Runs as a Kubernetes
// Job (Helm pre-upgrade hook) so the schema is always ahead of the code.
package main

import (
	"errors"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/abrifuh6/buam-pulse/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Error("DATABASE_URL required")
		os.Exit(1)
	}
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		log.Error("source", "err", err)
		os.Exit(1)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, url)
	if err != nil {
		log.Error("connect", "err", err)
		os.Exit(1)
	}
	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}
	v, dirty, _ := m.Version()
	log.Info("migrations applied", "version", v, "dirty", dirty)
}
