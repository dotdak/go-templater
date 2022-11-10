.PHONY: linux64

linux64:
	GOOS=linux GOARCH=amd64 go build -o gotem .