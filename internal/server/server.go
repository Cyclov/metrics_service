package server

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/Cyclov/metrics_service/internal/config"
	"github.com/Cyclov/metrics_service/internal/config/db"
	"github.com/Cyclov/metrics_service/internal/handler"
	"github.com/Cyclov/metrics_service/internal/logger"
	"github.com/Cyclov/metrics_service/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func Run(ctx context.Context, cfg config.ServerSettings) error {
	var database *pgxpool.Pool
	var storage repository.Storage
	if cfg.DbAdr != "" {
		var err error
		database, err = db.Connect(ctx, cfg.DbAdr)
		if err != nil {
			return err
		}
		defer database.Close()
		storage = repository.NewPostgresStorage(database)
	}

	var fileStorage *repository.MemStorage
	if storage == nil {
		fileStorage = repository.NewMemStorage()
		storage = fileStorage
		if cfg.FileStoragePath != "" && cfg.Restore {
			if err := fileStorage.Load(cfg.FileStoragePath); err != nil {
				return err
			}
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var storeWG sync.WaitGroup
	defer storeWG.Wait()

	var onUpdate func() error
	if fileStorage != nil && cfg.FileStoragePath != "" && cfg.StoreInterval == 0 {
		onUpdate = func() error { return fileStorage.Save(cfg.FileStoragePath) }
	} else if fileStorage != nil && cfg.FileStoragePath != "" {
		storeWG.Add(1)
		go func() {
			defer storeWG.Done()
			storeMetrics(ctx, fileStorage, cfg.FileStoragePath, time.Duration(cfg.StoreInterval)*time.Second)
		}()
	}

	h := handler.New(storage, onUpdate)
	if database != nil {
		h = handler.New(storage, onUpdate, database)
	}
	router := chi.NewRouter()
	router.Use(logger.RequestLogger)
	router.Use(handler.GzipMiddleware)
	router.Post("/update/", h.Update)
	router.Post("/updates/", h.Updates)
	router.Post("/value/", h.Value)
	router.Post("/update/{type}/{name}/{value}", h.UpdatePath)
	router.Get("/value/{type}/{name}", h.ValuePath)
	router.Get("/ping", h.Ping)
	router.Get("/", h.AllMetrics)

	httpServer := &http.Server{
		Addr:    cfg.SrvAdr,
		Handler: router,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Log.Error("failed to gracefully shut down server", zap.Error(err))
		}
	}()

	if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func storeMetrics(ctx context.Context, storage *repository.MemStorage, path string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := storage.Save(path); err != nil {
				logger.Log.Error("failed to store metrics on shutdown", zap.Error(err))
			}
			return
		case <-ticker.C:
			if err := storage.Save(path); err != nil {
				logger.Log.Error("failed to store metrics", zap.Error(err))
			}
		}
	}
}
