package main

import (
	"net/http"

	"github.com/Cyclov/metrics_service/internal/config"
	"github.com/Cyclov/metrics_service/internal/handler"
	"github.com/Cyclov/metrics_service/internal/logger"
	"github.com/Cyclov/metrics_service/internal/repository"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func main() {

	if err := logger.Initialize("info"); err != nil {
		panic(err)
	}

	storage := repository.NewMemStorage()
	h := handler.New(storage)

	router := chi.NewRouter()
	router.Use(logger.RequestLogger)
	router.Post("/update", h.Update)
	router.Post("/value", h.Value)
	router.Post("/update/{type}/{name}/{value}", h.UpdatePath)
	router.Get("/value/{type}/{name}", h.ValuePath)
	router.Get("/", h.AllMetrics)

	logger.Log.Fatal("server can't start  ", zap.Error(http.ListenAndServe(config.ServerConfig().SrvAdr, router)))
}
