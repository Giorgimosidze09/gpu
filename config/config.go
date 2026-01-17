package config

import (
	"os"
)

// Config holds the application configuration
type Config struct {
	// Database
	DatabaseURL string

	// Server
	ServerPort string

	// AWS
	AWSRegion string

	// On-premise
	OnPremEndpoint string
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://localhost/gpu_orchestrator?sslmode=disable"),
		ServerPort:  getEnv("SERVER_PORT", "8080"),
		AWSRegion:   getEnv("AWS_REGION", "us-east-1"),
		OnPremEndpoint: getEnv("ONPREM_ENDPOINT", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
