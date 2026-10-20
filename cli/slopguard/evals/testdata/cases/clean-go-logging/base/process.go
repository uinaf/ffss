package batch

import "log"

func Process(items []string, handle func(string) error) int {
	done := 0
	for _, item := range items {
		if item == "" {
			continue
		}
		if handle(item) == nil {
			done++
		}
	}
	return done
}

var _ = log.Printf
