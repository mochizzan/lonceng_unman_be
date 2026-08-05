package entity

import "time"

// LoginRequest represents the input for an LMS login attempt.
type LoginRequest struct {
	NPM      string `json:"npm"`
	Password string `json:"password"`
}

// LoginResult represents the outcome of an LMS login attempt.
type LoginResult struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	NPM       string    `json:"npm"`
	Timestamp time.Time `json:"timestamp"`
}
