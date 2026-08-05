package service

import (
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
)

// HealthService implements health check business logic.
type HealthService struct {
	appName string
	env     string
}

// NewHealthService creates a HealthService from application config.
func NewHealthService(cfg config.AppConfig) *HealthService {
	return &HealthService{
		appName: cfg.Name,
		env:     cfg.Env,
	}
}

// Check returns the current health status of the service.
func (s *HealthService) Check() entity.HealthStatus {
	return entity.HealthStatus{
		Status:  "ok",
		Service: s.appName,
		Version: s.env,
	}
}
