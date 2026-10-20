package store

// Page returns the items for a 1-based page number.
func Page(items []string, page, size int) []string {
	if page < 1 || size < 1 {
		return nil
	}
	return items
}
