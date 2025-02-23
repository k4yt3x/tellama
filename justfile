build:
	go build -ldflags="-s -w" -trimpath -o bin/tellama ./cmd/tellama

debug:
	go build -o bin/tellama ./cmd/tellama

test:
    go test -v ./...
