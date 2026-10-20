package metrics

type Counter struct {
	counts map[string]int
}

func NewCounter() *Counter {
	return &Counter{counts: map[string]int{}}
}

func (c *Counter) Add(name string) {
	c.counts[name]++
}
