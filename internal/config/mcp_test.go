package config

import (
	"reflect"
	"testing"
)

func TestLoadMCP(t *testing.T) {
	tests := []struct {
		name      string
		mcpURL    string
		logtoURL  string
		missingDB string
		want      MCPConfig
		wantErr   bool
	}{
		{
			name:     "valid URLs",
			mcpURL:   "https://mcp.example.com",
			logtoURL: "https://logto.example.com",
			want: MCPConfig{
				MCPURL:      "https://mcp.example.com",
				LogtoURL:    "https://logto.example.com",
				DatabaseURL: "postgres://dbuser:p%40ss%3Aword@127.0.0.1:5432/app?sslmode=disable",
			},
		},
		{
			name:     "missing MCP_URL",
			logtoURL: "https://logto.example.com",
			wantErr:  true,
		},
		{
			name:    "missing LOGTO_URL",
			mcpURL:  "https://mcp.example.com",
			wantErr: true,
		},
		{
			name:     "relative MCP_URL",
			mcpURL:   "/mcp",
			logtoURL: "https://logto.example.com",
			wantErr:  true,
		},
		{
			name:     "relative LOGTO_URL",
			mcpURL:   "https://mcp.example.com",
			logtoURL: "/logto",
			wantErr:  true,
		},
		{
			name:     "URL without host",
			mcpURL:   "https:///mcp",
			logtoURL: "https://logto.example.com",
			wantErr:  true,
		},
		{
			name:     "malformed URL",
			mcpURL:   "https://mcp.example.com",
			logtoURL: "https://logto.example.com/%ZZ",
			wantErr:  true,
		},
		{
			name:      "missing APP_DB",
			mcpURL:    "https://mcp.example.com",
			logtoURL:  "https://logto.example.com",
			missingDB: "APP_DB",
			wantErr:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("MCP_URL", test.mcpURL)
			t.Setenv("LOGTO_URL", test.logtoURL)
			t.Setenv("POSTGRES_USER", "dbuser")
			t.Setenv("POSTGRES_PASSWORD", "p@ss:word")
			t.Setenv("APP_DB", "app")
			if test.missingDB != "" {
				t.Setenv(test.missingDB, "")
			}

			configuration, err := LoadMCP()
			if (err != nil) != test.wantErr {
				t.Errorf("LoadMCP() error = %v, wantErr %v", err, test.wantErr)
			}
			if !reflect.DeepEqual(configuration, test.want) {
				t.Errorf("LoadMCP() = %v, want %v", configuration, test.want)
			}
		})
	}
}
