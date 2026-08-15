package calculator

import (
	"io"
	"os"
)

func ReadConfig(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(file)
}
