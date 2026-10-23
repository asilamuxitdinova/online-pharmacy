// Command pharmacy runs the Test Pharmacy API.
//
//	pharmacy serve     migrate, seed an empty database, and serve (default)
//	pharmacy migrate   create or update the schema
//	pharmacy seed      seed the database if it is empty
//	pharmacy reset     wipe all data and seed it again
//	pharmacy version   print the version
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/api"
	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/config"
	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/store"
)

var version = "dev"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	if err := run(cmd, log); err != nil {
		log.Error("exit", "cmd", cmd, "err", err)
		os.Exit(1)
	}
}

func run(cmd string, log *slog.Logger) error {
	if cmd == "version" {
		fmt.Println(version)
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	now := time.Now().Unix()

	switch cmd {
	case "migrate":
		log.Info("schema up to date", "db", cfg.DatabasePath)
		return nil
	case "seed":
		seeded, err := st.SeedIfEmpty(ctx, now)
		log.Info("seed", "seeded", seeded)
		return err
	case "reset":
		if err := st.Reset(ctx, now); err != nil {
			return err
		}
		log.Info("data reset", "db", cfg.DatabasePath)
		return nil
	case "serve":
		return serve(ctx, cfg, st, log)
	default:
		return fmt.Errorf("unknown command %q (serve, migrate, seed, reset, version)", cmd)
	}
}

func serve(ctx context.Context, cfg config.Config, st *store.Store, log *slog.Logger) error {
	if seeded, err := st.SeedIfEmpty(ctx, time.Now().Unix()); err != nil {
		return err
	} else if seeded {
		log.Info("seeded an empty database")
	}
	api.Version = version
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.New(cfg, st, log),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr, "public_url", cfg.PublicURL, "version", version)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
