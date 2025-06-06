package controller

import (
	"net/url"
	"os"

	"github.com/jmoiron/sqlx"
)

type Controller struct {
	db          *sqlx.DB
	frontendUrl string
}

func New(db *sqlx.DB) *Controller {
	frontendUrl, err := url.Parse(os.Getenv("FRONTEND_URL"))
	if err != nil {
		panic("Invalid FRONTEND_URL configuration")
	}

	// Load frontend URL from config
	if frontendUrl.String() == "" {
		panic("FRONTEND_URL configuration is missing")
	}

	return &Controller{
		db:          db,
		frontendUrl: frontendUrl.String(),
	}
}
