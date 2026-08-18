package server

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/Cyclov/metrics_service/internal/config"
	"github.com/Cyclov/metrics_service/internal/handler"
	"github.com/Cyclov/metrics_service/internal/logger"
	"github.com/Cyclov/metrics_service/internal/repository"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func Run(ctx context.Context, cfg config.ServerSettings) error {
	storage := repository.NewMemStorage()
	if cfg.Restore {
		if err := storage.Load(cfg.FileStoragePath); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var storeWG sync.WaitGroup
	defer storeWG.Wait()

	var onUpdate func() error
	if cfg.StoreInterval == 0 {
		onUpdate = func() error { return storage.Save(cfg.FileStoragePath) }
	} else {
		storeWG.Add(1)
		go func() {
			defer storeWG.Done()
			storeMetrics(ctx, storage, cfg.FileStoragePath, cfg.StoreInterval)
		}()
	}

	h := handler.New(storage, onUpdate)
	router := chi.NewRouter()
	router.Use(logger.RequestLogger)
	router.Use(handler.GzipMiddleware)
	router.Post("/update/", h.Update)
	router.Post("/value/", h.Value)
	router.Post("/update/{type}/{name}/{value}", h.UpdatePath)
	router.Get("/value/{type}/{name}", h.ValuePath)
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
