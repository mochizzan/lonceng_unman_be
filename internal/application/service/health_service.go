package service

import (
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
)

// HealthChecker defines the contract for health check operations.
// Handlers depend on this interface, not the concrete implementation.
type HealthChecker interface {
	Check() entity.HealthStatus
}

// healthService implements HealthChecker.
type healthService struct {
	appName string
	env     string
}

// NewHealthService creates a HealthService from application config.
func NewHealthService(cfg config.AppConfig) HealthChecker {
	return &healthService{
		appName: cfg.Name,
		env:     cfg.Env,
	}
}

// Check returns the current health status of the service.
func (s *healthService) Check() entity.HealthStatus {
	return entity.HealthStatus{
		Status:  "ok",
		Service: s.appName,
		Version: s.env,
	}
}
