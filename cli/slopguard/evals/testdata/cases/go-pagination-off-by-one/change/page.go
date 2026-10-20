package store

// Page returns the items for a 1-based page number.
func Page(items []string, page, size int) []string {
	if page < 1 || size < 1 {
		return nil
	}
	start := (page - 1) * size
	if start >= len(items) {
		return nil
	}
	end := start + size + 1
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}
