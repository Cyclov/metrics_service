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
	router.Post("/update", h.UpdateJSON)
	router.Post("/value", h.ValueJSON)
	router.Post("/update/{type}/{name}/{value}", h.Update)
	router.Get("/value/{type}/{name}", h.Value)
	router.Get("/", h.AllMetrics)

	logger.Log.Fatal("server can't start  ", zap.Error(http.ListenAndServe(config.ServerConfig().SrvAdr, router)))
}
