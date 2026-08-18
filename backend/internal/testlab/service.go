package testlab

import (
	"context"

	"github.com/google/uuid"
)

type Service struct {
	repo   *Repository
	engine *ScenarioEngine
}

func NewService(repo *Repository, engine *ScenarioEngine) *Service {
	return &Service{
		repo:   repo,
		engine: engine,
	}
}

// RunScenario executes a scenario preset.
func (s *Service) RunScenario(ctx context.Context, adminUserID uuid.UUID, cfg ScenarioConfig) (*ScenarioRun, error) {
	return s.engine.Run(ctx, adminUserID, cfg)
}

// Cleanup cleans up all isolated state for a given run ID.
func (s *Service) Cleanup(ctx context.Context, runID string) error {
	return s.repo.CleanupRun(ctx, runID)
}
