package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/0xD34DC0D3/template/internal/config"
	"github.com/0xD34DC0D3/template/internal/db"
	"github.com/0xD34DC0D3/template/migrations"
)

func runMigrate(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: appname migrate <up|down|status>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer d.Close()

	// Migrations always go through the single writer handle.
	p, err := newProvider(d)
	if err != nil {
		return err
	}
	ctx := context.Background()
	switch args[0] {
	case "up":
		_, err = p.Up(ctx)
		return err
	case "down":
		_, err = p.Down(ctx)
		return err
	case "status":
		return printStatus(ctx, p)
	default:
		return fmt.Errorf("unknown migrate command %q (want up|down|status)", args[0])
	}
}

// newProvider uses goose's instance API rather than its package-level
// globals, so the embedded FS and dialect are not process-wide state.
func newProvider(d *db.DB) (*goose.Provider, error) {
	return goose.NewProvider(goose.DialectSQLite3, d.Writer, migrations.FS)
}

func printStatus(ctx context.Context, p *goose.Provider) error {
	st, err := p.Status(ctx)
	if err != nil {
		return err
	}
	for _, s := range st {
		applied := "pending"
		if s.State == goose.StateApplied {
			applied = "applied " + s.AppliedAt.Format(time.RFC3339)
		}
		fmt.Fprintf(os.Stdout, "%-40s %s\n", s.Source.Path, applied)
	}
	return nil
}

// requireMigrated fails when any migration is pending. serve and doctor share
// it so both refuse the same states.
func requireMigrated(ctx context.Context, p *goose.Provider) error {
	pending, err := p.HasPending(ctx)
	if err != nil {
		return err
	}
	if pending {
		return errors.New("migrations pending; run 'appname migrate up'")
	}
	return nil
}
