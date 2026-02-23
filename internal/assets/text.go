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

func extractStringLiterals(content string) []string {
	results := make([]string, 0, 128)
	for i := 0; i < len(content); i++ {
		if content[i] != '"' {
			continue
		}

		start := i + 1
		j := start
		for ; j < len(content); j++ {
			if content[j] == '\\' {
				j++
				continue
			}
			if content[j] == '"' {
				break
			}
		}
		if j >= len(content) || j <= start {
			continue
		}
		results = append(results, content[start:j])
		i = j
	}
	return results
}
