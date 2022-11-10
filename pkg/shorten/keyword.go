package shorten

import "strings"

var (
	Dictionary = map[string]string{
		"context":  "ctx",
		"request":  "req",
		"response": "res",
	}
)

func Lookup(word string) string {
	word = strings.ToLower(word)
	if v, ok := Dictionary[word]; ok {
		return v
	}

	if strings.HasSuffix(word, "request") {
		return Dictionary["request"]
	}

	if strings.HasSuffix(word, "response") {
		return Dictionary["response"]
	}

	return word
}
