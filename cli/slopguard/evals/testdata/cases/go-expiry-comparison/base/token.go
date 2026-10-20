package auth

import "time"

type Token struct {
	Value     string
	ExpiresAt time.Time
}

// Valid reports whether the token can still be used at now.
func (t Token) Valid(now time.Time) bool {
	return t.Value != ""
}
