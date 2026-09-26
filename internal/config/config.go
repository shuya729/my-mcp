package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	MCPURL   string
	LogtoURL string
}

// 環境変数を読み込み Config を返す
func Load() (Config, error) {
	// 環境変数を読み込み
	mcpURL := strings.TrimSpace(os.Getenv("MCP_URL"))
	logtoURL := strings.TrimSpace(os.Getenv("LOGTO_URL"))

	// 環境変数の必須チェック
	if mcpURL == "" {
		return Config{}, errors.New("MCP_URL is required")
	}
	if logtoURL == "" {
		return Config{}, errors.New("LOGTO_URL is required")
	}

	// 環境変数の形式チェック
	if _, err := parseAbsoluteURL(mcpURL); err != nil {
		return Config{}, fmt.Errorf("MCP_URL: %w", err)
	}
	if _, err := parseAbsoluteURL(logtoURL); err != nil {
		return Config{}, fmt.Errorf("LOGTO_URL: %w", err)
	}

	return Config{
		MCPURL:   mcpURL,
		LogtoURL: logtoURL,
	}, nil
}

func parseAbsoluteURL(value string) (*url.URL, error) {
	parsedURL, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, errors.New("URL must be absolute")
	}
	return parsedURL, nil
}
