package metrics

// Reset clears every count.
func (c *Counter) Reset() {
	c.counts = nil
}
