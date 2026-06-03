package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lxmwaniky/mpesa-service-go/config"
	delivery "github.com/lxmwaniky/mpesa-service-go/internal/delivery/http"
	"github.com/lxmwaniky/mpesa-service-go/internal/infrastructure/daraja"
	"github.com/lxmwaniky/mpesa-service-go/internal/infrastructure/repository"
	"github.com/lxmwaniky/mpesa-service-go/internal/usecase"
)

func main() {
	config.LoadEnv()
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	var logger *slog.Logger
	if cfg.MpesaEnv == "production" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	slog.SetDefault(logger)

	pgDB, err := repository.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to initialize postgres connection", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := pgDB.Close(); err != nil {
			slog.Error("failed to close database safely", "err", err)
		}
	}()

	txRepo := repository.NewPostgresTransactionRepository(pgDB.DB)
	darajaClient := daraja.NewClient(cfg)
	mpesaUsecase := usecase.NewMpesaUsecase(txRepo, darajaClient)
	router := delivery.NewRouter(mpesaUsecase, cfg)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server listener failed", "err", err)
			os.Exit(1)
		}
	}()

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-shutdownChan
	slog.Warn("received termination signal", "signal", sig.String())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server graceful shutdown failed", "err", err)
	} else {
		slog.Info("server stopped gracefully")
	}
}