package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type MCPConfig struct {
	MCPURL      string
	LogtoURL    string
	DatabaseURL string
}

// MCP 用の環境変数を読み込む
func LoadMCP() (MCPConfig, error) {
	// 環境変数を読み込み
	mcpURL := strings.TrimSpace(os.Getenv("MCP_URL"))
	logtoURL := strings.TrimSpace(os.Getenv("LOGTO_URL"))

	// 環境変数の必須チェック
	if mcpURL == "" {
		return MCPConfig{}, errors.New("MCP_URL is required")
	}
	if logtoURL == "" {
		return MCPConfig{}, errors.New("LOGTO_URL is required")
	}

	// 環境変数の形式チェック
	if _, err := parseAbsoluteURL(mcpURL); err != nil {
		return MCPConfig{}, fmt.Errorf("MCP_URL: %w", err)
	}
	if _, err := parseAbsoluteURL(logtoURL); err != nil {
		return MCPConfig{}, fmt.Errorf("LOGTO_URL: %w", err)
	}
	databaseURL, err := databaseURLFromEnv()
	if err != nil {
		return MCPConfig{}, err
	}

	return MCPConfig{
		MCPURL:      mcpURL,
		LogtoURL:    logtoURL,
		DatabaseURL: databaseURL,
	}, nil
}
