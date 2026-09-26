package config

import (
	"reflect"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		mcpURL   string
		logtoURL string
		want     Config
		wantErr  bool
	}{
		{
			name:     "valid URLs",
			mcpURL:   "https://mcp.example.com",
			logtoURL: "https://logto.example.com",
			want: Config{
				MCPURL:   "https://mcp.example.com",
				LogtoURL: "https://logto.example.com",
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("MCP_URL", test.mcpURL)
			t.Setenv("LOGTO_URL", test.logtoURL)

			configuration, err := Load()
			if (err != nil) != test.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, test.wantErr)
			}
			if !reflect.DeepEqual(configuration, test.want) {
				t.Errorf("Load() = %v, want %v", configuration, test.want)
			}
		})
	}
}
