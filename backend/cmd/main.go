package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/fazamuttaqien/calendly/database"
	"github.com/fazamuttaqien/calendly/internal/presenter"
	"github.com/fazamuttaqien/calendly/internal/router"
)

func main() {
	dbUrl := os.Getenv("POSTGRES_URL")
	db, err := database.New(dbUrl)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		return
	}
	defer db.Close()

	presenter := presenter.New(db.DB)
	router := router.New(presenter)

	slog.Info("Starting server on :8000...")
	http.ListenAndServe(":8000", router)
}
