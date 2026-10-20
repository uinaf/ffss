package batch

import "log"

func Process(items []string, handle func(string) error) int {
	done := 0
	for index, item := range items {
		if item == "" {
			log.Printf("skipping empty item at index %d", index)
			continue
		}
		if handle(item) == nil {
			done++
		}
	}
	return done
}
