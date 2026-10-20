package profile

type Store interface {
	Load(id string) (map[string]string, error)
	Save(id string, values map[string]string) error
}

// Rename sets the display name for a profile.
func Rename(store Store, id, name string) error {
	values, err := store.Load(id)
	if err != nil {
		return err
	}
	values["name"] = name
	return store.Save(id, values)
}
