package config

import (
	"errors"
	"net/url"
	"os"
)

func databaseURLFromEnv() (string, error) {
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	database := os.Getenv("APP_DB")
	if user == "" || password == "" || database == "" {
		return "", errors.New("POSTGRES_USER, POSTGRES_PASSWORD, and APP_DB are required")
	}
	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     "127.0.0.1:5432",
		Path:     "/" + database,
		RawPath:  "/" + url.PathEscape(database),
		RawQuery: "sslmode=disable",
	}).String(), nil
}
