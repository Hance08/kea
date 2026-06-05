build:
	go build ./cmd/kea

run:
	go run ./cmd/kea

test-race:
	go test -race ./...
