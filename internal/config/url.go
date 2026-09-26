package config

import (
	"errors"
	"fmt"
	"net/url"
)

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
