package main

import (
	"log"
	"net/http"

	"github.com/Cyclov/metrics_service/internal/handler"
	"github.com/Cyclov/metrics_service/internal/repository"
)

func main() {
	storage := repository.NewMemStorage()
	h := handler.New(storage)

	mux := http.NewServeMux()
	mux.HandleFunc("/update/{type}/{name}/{value}", h.Update)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
