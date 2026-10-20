package files

import "os"

// TotalSize returns the combined size of the named files.
func TotalSize(paths []string) (int64, error) {
	var total int64
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return 0, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return 0, err
		}
		total += info.Size()
	}
	return total, nil
}
