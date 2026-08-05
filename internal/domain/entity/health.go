package entity

// HealthStatus represents the health state of the service.
type HealthStatus struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}
