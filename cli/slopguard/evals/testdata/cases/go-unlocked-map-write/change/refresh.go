package rates

import "time"

type Source interface {
	Fetch() (map[string]float64, error)
}

// Refresh reloads every rate from source on each tick until stop closes.
func (c *Cache) Refresh(source Source, every time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(every)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				latest, err := source.Fetch()
				if err != nil {
					continue
				}
				for currency, rate := range latest {
					c.rates[currency] = rate
				}
			}
		}
	}()
}
