package assets

import (
	"fmt"
	"os"
	"unicode/utf8"
)

func osReadFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return "", fmt.Errorf("invalid UTF-8 encoding in %s", path)
	}
	return string(b), nil
}
