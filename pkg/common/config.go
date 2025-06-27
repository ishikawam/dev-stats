package common

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds common configuration
type Config struct {
	StartDate time.Time
	EndDate   time.Time
}

// LoadConfig loads common configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	godotenv.Load()

	startDateStr := os.Getenv("START_DATE")
	endDateStr := os.Getenv("END_DATE")

	if startDateStr == "" || endDateStr == "" {
		return nil, NewError("Environment variables START_DATE and END_DATE must be set")
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return nil, NewError("Invalid START_DATE format: %v", err)
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return nil, NewError("Invalid END_DATE format: %v", err)
	}

	return &Config{
		StartDate: startDate,
		EndDate:   endDate,
	}, nil
}
