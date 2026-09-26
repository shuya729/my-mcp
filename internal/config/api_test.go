package config

import (
	"reflect"
	"testing"
)

func TestLoadAPI(t *testing.T) {
	tests := []struct {
		name      string
		apiURL    string
		logtoURL  string
		missingDB string
		want      APIConfig
		wantErr   bool
	}{
		{
			name:     "valid configuration",
			apiURL:   "https://api.example.com",
			logtoURL: "https://logto.example.com",
			want: APIConfig{
				APIURL:      "https://api.example.com",
				LogtoURL:    "https://logto.example.com",
				DatabaseURL: "postgres://dbuser:p%40ss%3Aword@127.0.0.1:5432/app?sslmode=disable",
			},
		},
		{name: "missing API_URL", logtoURL: "https://logto.example.com", wantErr: true},
		{name: "missing LOGTO_URL", apiURL: "https://api.example.com", wantErr: true},
		{name: "invalid API_URL", apiURL: "/api", logtoURL: "https://logto.example.com", wantErr: true},
		{name: "invalid LOGTO_URL", apiURL: "https://api.example.com", logtoURL: "/logto", wantErr: true},
		{name: "missing POSTGRES_USER", apiURL: "https://api.example.com", logtoURL: "https://logto.example.com", missingDB: "POSTGRES_USER", wantErr: true},
		{name: "missing POSTGRES_PASSWORD", apiURL: "https://api.example.com", logtoURL: "https://logto.example.com", missingDB: "POSTGRES_PASSWORD", wantErr: true},
		{name: "missing APP_DB", apiURL: "https://api.example.com", logtoURL: "https://logto.example.com", missingDB: "APP_DB", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("API_URL", tt.apiURL)
			t.Setenv("LOGTO_URL", tt.logtoURL)
			t.Setenv("POSTGRES_USER", "dbuser")
			t.Setenv("POSTGRES_PASSWORD", "p@ss:word")
			t.Setenv("APP_DB", "app")
			if tt.missingDB != "" {
				t.Setenv(tt.missingDB, "")
			}
			got, err := LoadAPI()
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("configuration = %+v, want %+v", got, tt.want)
			}
		})
	}
}
