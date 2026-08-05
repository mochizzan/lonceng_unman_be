package service_test

import (
	"testing"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
)

func TestHealthService_Check(t *testing.T) {
	cfg := config.AppConfig{
		Name: "test-service",
		Env:  "testing",
	}

	svc := service.NewHealthService(cfg)
	status := svc.Check()

	if status.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", status.Status)
	}
	if status.Service != "test-service" {
		t.Errorf("expected service 'test-service', got %q", status.Service)
	}
	if status.Version != "testing" {
		t.Errorf("expected version 'testing', got %q", status.Version)
	}
}

func TestHealthService_ImplementsInterface(t *testing.T) {
	cfg := config.AppConfig{Name: "x", Env: "x"}
	var _ service.HealthChecker = service.NewHealthService(cfg)
}
