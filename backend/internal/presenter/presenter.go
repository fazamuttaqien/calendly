package presenter

import (
	"github.com/fazamuttaqien/calendly/internal/controller"
	"github.com/jmoiron/sqlx"
)

type Presenter struct {
	Controllers *controller.Controller
}

func New(db *sqlx.DB) Presenter {
	controllers := controller.New(db)
	return Presenter{
		Controllers: controllers,
	}
}
