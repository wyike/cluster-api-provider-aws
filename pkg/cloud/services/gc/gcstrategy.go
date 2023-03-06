package gc

import (
	"context"
	"fmt"
)

type gcStrategy interface {
	Cleanup(ctx context.Context) error
}

func newDefaultGcStrategy(collect ResourceCollectFuncs, cleanup ResourceCleanupFuncs) *defaultGcStrategy {
	gs := &defaultGcStrategy{
		collects: collect,
		cleanups: cleanup,
	}

	return gs
}

func newSecondaryGcStrategy(collect ResourceCollectFuncs, cleanup ResourceCleanupFuncs) *SecondaryGcStrategy {
	gs := &SecondaryGcStrategy{
		collects: collect,
		cleanups: cleanup,
	}

	return gs
}

type defaultGcStrategy struct {
	collects ResourceCollectFuncs
	cleanups ResourceCleanupFuncs
}

type SecondaryGcStrategy struct {
	collects ResourceCollectFuncs
	cleanups ResourceCleanupFuncs
}

func (s *defaultGcStrategy) Cleanup(ctx context.Context) error {
	resources, err := s.collects.Execute(ctx)
	if err != nil {
		return err
	}

	if deleteErr := s.cleanups.Execute(ctx, resources); deleteErr != nil {
		return fmt.Errorf("deleting resources: %w", deleteErr)
	}

	return nil
}

func (s *SecondaryGcStrategy) Cleanup(ctx context.Context) error {
	resources, err := s.collects.Execute(ctx)
	if err != nil {
		return err
	}

	if deleteErr := s.cleanups.Execute(ctx, resources); deleteErr != nil {
		return fmt.Errorf("deleting resources: %w", deleteErr)
	}

	return nil
}
