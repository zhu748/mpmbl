package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ds2api/internal/config"
	"ds2api/internal/server"
)

func main() {
	if err := config.LoadDotEnv(); err != nil {
		config.Logger.Warn("[dotenv] load failed", "error", err)
	}
	config.RefreshLogger()

	app, err := server.NewApp()
	if err != nil {
		config.Logger.Error("[server] init failed", "error", err)
		os.Exit(1)
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "5001"
	}
	addr := ":" + port
	srv := &http.Server{
		Addr:              addr,
		Handler:           app.Router,
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		config.Logger.Info("[server] listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			config.Logger.Error("[server] stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		config.Logger.Error("[server] graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	config.Logger.Info("[server] stopped")
}
