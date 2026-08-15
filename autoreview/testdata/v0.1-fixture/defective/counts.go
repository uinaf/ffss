package calculator

func CountByOwner(owners []string) map[string]int {
	var counts map[string]int
	for _, owner := range owners {
		counts[owner]++
	}
	return counts
}
