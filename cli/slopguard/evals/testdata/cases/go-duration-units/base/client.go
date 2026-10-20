package client

import (
	"net/http"
	"time"
)

type Config struct {
	TimeoutSeconds int
}

func New(config Config) *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}
