// Yo'ldosh AI — HTTP API serveri.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // vaqt zonalari binar ichida (konteynerda tzdata bo'lmasa ham ishlaydi)

	"yoldosh/internal/api"
	"yoldosh/internal/discover"
	"yoldosh/internal/platform"
	"yoldosh/internal/store"
	"yoldosh/static"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := platform.LoadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := platform.OpenDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := platform.Migrate(ctx, db); err != nil {
		return err
	}

	apiSrv := api.NewServer(cfg, store.New(db))
	apiSrv.SetImages(static.Places())
	apiSrv.SetCityImages(static.Cities())
	if os.Getenv("EXPO_PUSH_ENABLED") == "true" {
		go apiSrv.RunNudger(ctx, api.NewExpoPush(os.Getenv("EXPO_ACCESS_TOKEN")), 5*time.Minute)
		slog.Info("kechki eslatmalar yoqildi")
	}
	// Yangi joylarni avtomatik topish: Wikidata (CC0) dan har DISCOVERY_EVERY da (masalan 24h). Bo'sh — o'chiq.
	if v := os.Getenv("DISCOVERY_EVERY"); v != "" {
		every, err := time.ParseDuration(v)
		if err != nil || every < time.Hour {
			return errors.New("DISCOVERY_EVERY: kamida 1h (masalan 24h)")
		}
		if ua := os.Getenv("DISCOVERY_USER_AGENT"); ua != "" {
			discover.UserAgent = ua
		}
		go discover.Schedule(ctx, db, slog.Default(), 2*time.Minute, every)
		slog.Info("joy topish yoqildi", "every", every)
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           apiSrv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		slog.Info("server ishga tushdi", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	slog.Info("to'xtatilmoqda")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
