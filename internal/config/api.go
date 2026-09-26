package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type APIConfig struct {
	APIURL      string
	LogtoURL    string
	DatabaseURL string
}

func LoadAPI() (APIConfig, error) {
	apiURL := strings.TrimSpace(os.Getenv("API_URL"))
	logtoURL := strings.TrimSpace(os.Getenv("LOGTO_URL"))
	if apiURL == "" || logtoURL == "" {
		return APIConfig{}, errors.New("API_URL and LOGTO_URL are required")
	}
	if _, err := parseAbsoluteURL(apiURL); err != nil {
		return APIConfig{}, fmt.Errorf("API_URL: %w", err)
	}
	if _, err := parseAbsoluteURL(logtoURL); err != nil {
		return APIConfig{}, fmt.Errorf("LOGTO_URL: %w", err)
	}
	databaseURL, err := databaseURLFromEnv()
	if err != nil {
		return APIConfig{}, err
	}
	return APIConfig{APIURL: apiURL, LogtoURL: logtoURL, DatabaseURL: databaseURL}, nil
}
