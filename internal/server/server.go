package server

import (
	"net/http"
	"time"

	"github.com/Cyclov/metrics_service/internal/config"
	"github.com/Cyclov/metrics_service/internal/handler"
	"github.com/Cyclov/metrics_service/internal/logger"
	"github.com/Cyclov/metrics_service/internal/repository"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func Run(cfg config.ServerSettings) error {
	storage := repository.NewMemStorage()
	if cfg.Restore {
		if err := storage.Load(cfg.FileStoragePath); err != nil {
			return err
		}
	}

	var onUpdate func() error
	if cfg.StoreInterval == 0 {
		onUpdate = func() error { return storage.Save(cfg.FileStoragePath) }
	} else {
		go storeMetrics(storage, cfg.FileStoragePath, cfg.StoreInterval)
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

	return http.ListenAndServe(cfg.SrvAdr, router)
}

func storeMetrics(storage *repository.MemStorage, path string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := storage.Save(path); err != nil {
			logger.Log.Error("failed to store metrics", zap.Error(err))
		}
	}
}
