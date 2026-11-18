package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/inngest/inngestgo/pkg/env"

	"github.com/0xD34DC0D3/template/internal/config"
	"github.com/0xD34DC0D3/template/internal/db"
)

// runDoctor runs the same checks serve does at startup, so a bad deploy
// config or an un-migrated database is caught without starting the listener.
func runDoctor(_ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.CheckServe(); err != nil {
		return err
	}
	// A dev machine often runs doctor without voodoo up; only a deploy must
	// be able to reach it.
	if err := checkVoodoo(cfg.VoodooURL); err != nil {
		if cfg.Prod {
			return err
		}
		fmt.Println("WARN:", err)
	}
	// CheckServe already required the keys in prod; in dev, say why jobs
	// would silently not run.
	if !cfg.Prod {
		if err := checkInngestDev(cfg); err != nil {
			fmt.Println("WARN:", err)
		}
	}
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer d.Close()

	ctx := context.Background()
	p, err := newProvider(d)
	if err != nil {
		return err
	}
	if err := printStatus(ctx, p); err != nil {
		return err
	}
	if err := requireMigrated(ctx, p); err != nil {
		return err
	}
	fmt.Println("OK")
	return nil
}

// checkVoodoo confirms the auth server answers, since without it no one can
// sign in and every protected route answers 503.
func checkVoodoo(base string) error {
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get(base + "/healthz")
	if err != nil {
		return fmt.Errorf("voodoo unreachable: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("voodoo %s/healthz answered %d, want 200", base, resp.StatusCode)
	}
	return nil
}

// checkInngestDev reports a dev setup in which jobs cannot run: neither the
// dev server nor Inngest Cloud keys, or a dev server that is not up. The URL
// comes from the SDK so both agree on where the dev server is.
func checkInngestDev(cfg *config.Config) error {
	if cfg.InngestDev == "" {
		if cfg.InngestSigningKey == "" {
			return errors.New("INNGEST_DEV and INNGEST_SIGNING_KEY are unset; jobs will not run (set INNGEST_DEV=1 for the dev server)")
		}
		return nil
	}
	u := env.DevServerURL()
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get(u)
	if err != nil {
		return fmt.Errorf("inngest dev server %s unreachable (see AGENTS.md to start it): %w", u, err)
	}
	resp.Body.Close()
	return nil
}
