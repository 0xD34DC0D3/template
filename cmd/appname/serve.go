package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/0xD34DC0D3/voodoo/voodooauth"

	"github.com/0xD34DC0D3/template/internal/config"
	"github.com/0xD34DC0D3/template/internal/db"
	"github.com/0xD34DC0D3/template/internal/jobs"
	"github.com/0xD34DC0D3/template/internal/server"
)

func runServe(_ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.CheckServe(); err != nil {
		return err
	}
	slog.SetLogLoggerLevel(cfg.LogLevel)

	d, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer d.Close()
	// Refuse to serve against an un-migrated schema; same check as doctor.
	p, err := newProvider(d)
	if err != nil {
		return err
	}
	if err := requireMigrated(context.Background(), p); err != nil {
		return err
	}

	auth := voodooauth.New(cfg.VoodooURL)
	// Empty only in dev (CheckServe requires it in prod); voodooauth then
	// falls back to the Host header for next=.
	auth.AppURL = cfg.BaseURL

	jc, err := jobs.New(jobs.Deps{R: db.New(d.Reader), W: db.New(d.Writer)})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.BindHost, strconv.Itoa(cfg.Port)),
		Handler:           server.New(d, auth, jc, cfg.SiteURL),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		// Matches WriteTimeout so an in-flight Inngest step can finish.
		shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
