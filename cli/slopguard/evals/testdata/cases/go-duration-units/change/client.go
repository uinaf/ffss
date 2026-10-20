package client

import (
	"net/http"
	"time"
)

type Config struct {
	TimeoutSeconds int
}

func New(config Config) *http.Client {
	timeout := 30 * time.Second
	if config.TimeoutSeconds > 0 {
		timeout = time.Duration(config.TimeoutSeconds)
	}
	return &http.Client{Timeout: timeout}
}
