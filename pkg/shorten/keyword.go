package shorten

import (
	"strings"
)

var (
	Dictionary = map[string]string{
		"context":  "ctx",
		"request":  "req",
		"response": "res",
		"error":    "err",
		"services": "svcs",
		"service":  "svc",
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

func TrimFileName(name string) (out string) {
	out = name
	out = strings.TrimSuffix(out, ".go")
	out = strings.TrimSuffix(out, ".pb")
	out = strings.TrimSuffix(out, ".gw")
	out = strings.TrimSuffix(out, ".connect")
	out = strings.TrimSuffix(out, "_grpc")
	out = strings.TrimSuffix(out, "_service")
	return
}

func LowerFirst(name string) string {
	if len(name) < 2 {
		return strings.ToLower(name)
	}

	return strings.ToLower(name[0:1]) + name[1:]
}

func TrimServiceName(name string) (out string) {
	out = name
	out = strings.TrimSuffix(out, "Server")
	out = strings.TrimSuffix(out, "Service")
	return
}
