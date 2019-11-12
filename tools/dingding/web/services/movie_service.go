package services

import (
	"log"
	"time"

	"github.com/codelieche/cronjob/tools/dingding/datasource"

	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/codelieche/cronjob/tools/dingding/repositories"
)

type MovieService interface {
	GetAll() []*datamodels.Movie
	GetByID(id int64) (*datamodels.Movie, bool)
	PostCreate() *datamodels.Movie
	DeleteByID(id int64) (ok bool)
}

func NewMoviewService(repo repositories.MovieRepository) MovieService {
	return &movieService{
		repo: repo,
	}
}

type movieService struct {
	repo repositories.MovieRepository
}

func (s *movieService) GetAll() []*datamodels.Movie {
	return s.repo.GetAll()
}

func (s *movieService) GetByID(id int64) (movie *datamodels.Movie, ok bool) {
	log.Println(id)
	return s.repo.GetById(id)
}

func (s *movieService) PostCreate() (movie *datamodels.Movie) {
	// 对id加1
	datasource.MovieID++

	movie = &datamodels.Movie{
		Title:       "Create Is OK",
		Description: "简单的描述信息",
	}
	movie.Model.ID = uint(datasource.MovieID)
	movie.Model.CreatedAt = time.Now()
	movie.Model.UpdatedAt = time.Now()

	datasource.Movies[datasource.MovieID] = movie

	return movie
}

func (s *movieService) DeleteByID(id int64) (ok bool) {
	delete(datasource.Movies, id)
	return true
}

func (s *movieService) GetHello() string {
	return "hello world!"
}

func (s *movieService) GetPing() string {
	return "pong"
}
