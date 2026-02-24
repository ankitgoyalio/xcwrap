package assets

import (
	"os"
	"unicode/utf8"
)

func osReadFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return "", nil
	}
	return string(b), nil
}
