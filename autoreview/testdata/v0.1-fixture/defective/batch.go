package calculator

func Batch(items []string, size int) [][]string {
	var batches [][]string
	for len(items) > 0 {
		batches = append(batches, items[:size])
		items = items[size:]
	}
	return batches
}
