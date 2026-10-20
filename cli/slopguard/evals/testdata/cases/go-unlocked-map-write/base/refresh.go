package rates

type Source interface {
	Fetch() (map[string]float64, error)
}
