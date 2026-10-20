package profile

type Store interface {
	Load(id string) (map[string]string, error)
	Save(id string, values map[string]string) error
}

// Rename sets the display name for a profile, skipping unchanged names.
func Rename(store Store, id, name string) error {
	values, err := store.Load(id)
	if err != nil {
		return err
	}
	if values["name"] != name {
		values["name"] = name
		if err := store.Save(id, values); err != nil {
			err = fmtError(id, err)
		}
	}
	return err
}

func fmtError(id string, err error) error {
	return &saveError{id: id, err: err}
}

type saveError struct {
	id  string
	err error
}

func (e *saveError) Error() string { return "save " + e.id + ": " + e.err.Error() }
