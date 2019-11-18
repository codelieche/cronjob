package repositories

import (
	"log"
	"sync"

	"github.com/codelieche/cronjob/tools/dingding/datamodels"
)

type MovieRepository interface {
	GetAll() []*datamodels.Movie
	GetById(int64) (movie *datamodels.Movie, ok bool)
}

func NewMovieRepository(source map[int64]*datamodels.Movie) MovieRepository {

	return &movieMemoryRepository{source: source}
}

type movieMemoryRepository struct {
	source map[int64]*datamodels.Movie
	mu     sync.RWMutex
}

func (r *movieMemoryRepository) GetAll() (allMovies []*datamodels.Movie) {
	// 获取锁
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, movie := range r.source {
		allMovies = append(allMovies, movie)
	}
	return allMovies

}

func (r *movieMemoryRepository) GetById(id int64) (movie *datamodels.Movie, ok bool) {
	// 获取锁
	log.Println(id)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if movie, isExit := r.source[id]; isExit {
		return movie, true
	} else {
		return nil, false
	}

}
