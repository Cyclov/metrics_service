package main

import (
	"log"
	"net/http"

	"github.com/Cyclov/metrics_service/internal/handler"
	"github.com/Cyclov/metrics_service/internal/repository"
	"github.com/go-chi/chi/v5"
)

func main() {
	storage := repository.NewMemStorage()
	h := handler.New(storage)

	router := chi.NewRouter()
	router.Post("/update/{type}/{name}/{value}", h.Update)
	router.Get("/value/{type}/{name}", h.Value)
	router.Get("/", h.AllMetrics)

	log.Fatal(http.ListenAndServe(":8080", router))
}
