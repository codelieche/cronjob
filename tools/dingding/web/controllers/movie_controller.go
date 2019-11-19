package controllers

import (
	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/codelieche/cronjob/tools/dingding/web/services"
)

type MovieController struct {
	Service services.MovieService
}

func (c *MovieController) Get() (results []*datamodels.Movie) {
	return c.Service.GetAll()
}

func (c *MovieController) PostCreate() (movie *datamodels.Movie) {
	return c.Service.PostCreate()
}

func (c *MovieController) GetAll() (results []*datamodels.Movie) {
	return c.Service.GetAll()
}

func (c *MovieController) GetBy(id int64) (movie *datamodels.Movie, found bool) {
	return c.Service.GetByID(id) // it will throw 404 if not found.
}

func (c *MovieController) DeleteBy(id int64) (ok bool) {
	return c.Service.DeleteByID(id) // it will throw 404 if not found.
}
