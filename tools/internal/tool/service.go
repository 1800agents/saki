package tool

import "context"

// Service hosts tool registrations. Tool handlers are added in follow-up issues.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
