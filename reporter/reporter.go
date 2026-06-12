package reporter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spaghettifunk/anima/internal/config"
	"github.com/spaghettifunk/anima/internal/storage/postgres"
)

type Reporter struct {
	engine *gin.Engine
	server *http.Server
	repo   *PostgresRepository
}

func New(cfg config.Config) (*Reporter, error) {
	if cfg.Reporter == nil {
		return nil, fmt.Errorf("reporter config is required")
	}

	store := postgres.NewFromConfig(cfg)
	repo := NewPostgresRepository(store)
	service := NewService(repo, Defaults{
		Tenant:              cfg.Reporter.DefaultTenant,
		LookbackDays:        cfg.Reporter.DefaultLookbackDays,
		Limit:               cfg.Reporter.DefaultLimit,
		MaxLimit:            cfg.Reporter.MaxLimit,
		MaxQueriedAtEntries: cfg.Reporter.MaxQueriedAtEntries,
	})
	engine := NewRouter(service)

	address := fmt.Sprintf("%s:%d", cfg.Reporter.Address, cfg.Reporter.Port)
	return &Reporter{
		engine: engine,
		server: &http.Server{
			Addr:              address,
			Handler:           engine,
			ReadHeaderTimeout: 5 * time.Second,
		},
		repo: repo,
	}, nil
}

func (r *Reporter) Initialize() error {
	return r.repo.Initialize(context.Background())
}

func (r *Reporter) Listen() error {
	if err := r.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (r *Reporter) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	if r.server != nil {
		err = r.server.Shutdown(ctx)
	}
	if r.repo != nil {
		r.repo.Close()
	}
	return err
}
