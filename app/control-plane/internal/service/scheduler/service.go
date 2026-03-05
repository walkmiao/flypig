package scheduler

import "context"

type Service struct{}

func New() *Service {
	return &Service{}
}

func (s *Service) Reconcile(_ context.Context) error {
	return nil
}
