package config

import (
	"fmt"
	"os"
)

// Config is the runtime configuration of the api service. It is read once at
// startup from the environment; the person who cloned the repo finds every
// variable named in RUN.json.
type Config struct {
	Port                  string
	DatabaseURL           string
	ValkeyURL             string
	WorkshopAdminEmail    string
	WorkshopAdminPassword string
	AuthTokenSecret       string
	WebOrigin             string
}

// Load reads the configuration from the environment. Every value the process
// needs to boot is required except PORT (which defaults to 8080) and WEB_ORIGIN
// (unset simply means no cross-origin access). A missing required value returns
// an error naming the variable, and main exits with that message.
func Load() (*Config, error) {
	required := []string{
		"DATABASE_URL",
		"VALKEY_URL",
		"WORKSHOP_ADMIN_EMAIL",
		"WORKSHOP_ADMIN_PASSWORD",
		"AUTH_TOKEN_SECRET",
	}
	for _, name := range required {
		if os.Getenv(name) == "" {
			return nil, fmt.Errorf("missing required environment variable %s (see RUN.json)", name)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		Port:                  port,
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		ValkeyURL:             os.Getenv("VALKEY_URL"),
		WorkshopAdminEmail:    os.Getenv("WORKSHOP_ADMIN_EMAIL"),
		WorkshopAdminPassword: os.Getenv("WORKSHOP_ADMIN_PASSWORD"),
		AuthTokenSecret:       os.Getenv("AUTH_TOKEN_SECRET"),
		WebOrigin:             os.Getenv("WEB_ORIGIN"),
	}, nil
}
