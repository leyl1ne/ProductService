package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/leyl1ne/ProductService/internal/config/app"
	postgresInfra "github.com/leyl1ne/ProductService/internal/infra/postgres"
	postgresRepo "github.com/leyl1ne/ProductService/internal/repository/postgres"
	batchservice "github.com/leyl1ne/ProductService/internal/service/batch"
	categoryservice "github.com/leyl1ne/ProductService/internal/service/category"
	productservice "github.com/leyl1ne/ProductService/internal/service/product"
	batchhandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/batch"
	categoryhandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/category"
	docshandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/docs"
	healthhandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/health"
	producthandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/product"
	"github.com/leyl1ne/ProductService/internal/transport/http/router"
	"github.com/leyl1ne/ProductService/pkg/logger"
	"github.com/leyl1ne/ProductService/pkg/logger/zl"
)

func main() {
	cfg, err := app.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	zeroLog, err := zl.NewZerologLogger(cfg.Logger)
	if err != nil {
		log.Fatalf("failed to init zero logger: %v", err)
	}

	zeroLog.Info("http server started!", logger.Field{Key: "Addr", Value: cfg.Server.HTTP.Port})
	zeroLog.Debug("logger debug mode enabled")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	db, err := postgresInfra.Open(ctx, cfg.Postgres)
	if err != nil {
		zeroLog.Fatal("failed to connect database", logger.Field{Key: "error", Value: err.Error()})
		os.Exit(1)
	}

	repo := postgresRepo.NewPostgresRepository(db.Pool())

	categoryService := categoryservice.NewService(repo)
	productService := productservice.NewService(repo)
	batchService := batchservice.NewService(repo)

	cateogryHandler := categoryhandler.NewHandler(zeroLog, categoryService)
	productHandler := producthandler.NewHandler(zeroLog, productService)
	batchHandler := batchhandler.NewHandler(zeroLog, batchService)
	healthHandler := healthhandler.NewHealthHandler(cfg.App.Name, cfg.App.Version, cfg.App.Environment)
	docsHandler, err := docshandler.NewDocsHandler()
	if err != nil {
		zeroLog.Fatal("failed to create docs handler", logger.Err(err))
	}

	router := router.SetupRouter(
		zeroLog,
		router.Handlers{
			CategoryHandler: cateogryHandler,
			ProductHandler:  productHandler,
			BatchHandler:    batchHandler,
			HealthHandler:   healthHandler,
			DocsHandler:     docsHandler,
		},
		cfg.Server,
	)

	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
		Handler:      router.Handler(),
		ReadTimeout:  cfg.Server.HTTP.ReadTimeout,
		WriteTimeout: cfg.Server.HTTP.WriteTimeout,
	}

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zeroLog.Fatal("server failed", logger.Err(err))
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.GracefulShutdown)
	defer shutdownCancel()

	if err := s.Shutdown(shutdownCtx); err != nil {
		zeroLog.Error("failed to stop server gracefully", logger.Err(err))

		if err := s.Close(); err != nil {
			zeroLog.Error("forced shutdown failed", logger.Err(err))
		}
		return

	}

	zeroLog.Info("server exiting")

	if err := db.Close(); err != nil {
		zeroLog.Error("failed to close database client", logger.Field{Key: "error", Value: err.Error()})
	}
}
