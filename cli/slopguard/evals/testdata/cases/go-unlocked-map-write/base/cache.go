package rates

import "sync"

type Cache struct {
	mu    sync.Mutex
	rates map[string]float64
}

func NewCache() *Cache {
	return &Cache{rates: map[string]float64{}}
}

func (c *Cache) Get(currency string) (float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rate, ok := c.rates[currency]
	return rate, ok
}

func (c *Cache) Set(currency string, rate float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rates[currency] = rate
}
