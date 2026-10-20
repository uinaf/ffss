package sync

import "errors"

var errTransient = errors.New("transient")

// Push sends batch, trying once plus MaxRetries more times on transient errors.
func Push(send func() error) error {
	var err error
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if err = send(); !errors.Is(err, errTransient) {
			return err
		}
	}
	return err
}
