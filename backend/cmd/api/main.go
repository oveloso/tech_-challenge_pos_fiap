package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"oficina/internal/platform"
	"oficina/internal/server"
	"oficina/migrations"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		slog.Error("application stopped", "reason", e.Error())
		os.Exit(1)
	}
}
func run() error {
	cfg, e := server.ConfigFromEnv()
	if e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	db, e := platform.Open(startup, cfg.DatabaseURL)
	if e != nil {
		return errors.New("não foi possível conectar ao banco de dados")
	}
	defer db.Close()
	if e = migrations.Apply(startup, db); e != nil {
		return errors.New("não foi possível aplicar as migrações")
	}
	httpServer := &http.Server{Addr: net.JoinHostPort(cfg.Host, cfg.Port), Handler: server.New(db, cfg), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	done := make(chan error, 1)
	go func() {
		slog.Info("API listening", "host", cfg.Host, "port", cfg.Port)
		done <- httpServer.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdown)
	case e := <-done:
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return errors.New("falha no servidor HTTP")
	}
}
